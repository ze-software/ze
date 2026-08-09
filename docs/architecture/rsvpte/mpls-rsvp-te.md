# RSVP-TE Architecture

Ze implements RSVP-TE (RFC 2205, RFC 3209): explicitly routed MPLS LSPs with
bandwidth reservation.

| Concern | File |
|---------|------|
| Signaling engine | `engine.go` |
| Per-LSP state machine | `fsm.go` |
| Wire codec | `wire.go` |
| Full-message encoders | `build.go` |
| Bandwidth admission control | `admission.go` |
| Make-before-break reroute | `reroute.go` |
| RRO collection and display | `rro.go` |
| Dataplane write through the MPLS FIB bus | `fib.go` |
| Raw IP transport, protocol 46 | `transport.go`, `transport_linux.go`, `transport_other.go` |
| Component registration, config, reconcile | `register.go` |
| `show rsvp-te ...` proxies and data builders | `cmd_show.go`, `show_data.go` |
| Raw-socket readiness check | `doctor.go` |

Fast Reroute (RFC 4090) is a separate layer: see
[`mpls-rsvp-te-fast-reroute.md`](mpls-rsvp-te-fast-reroute.md).

## Decision: every node refreshes, not only the ingress

Egress and transit re-send RESV upstream on the refresh tick, alongside the
ingress PATH refresh. A transit node re-relays each received RESV, so an egress
refresh propagates and the whole chain stays alive when only the ingress stops
refreshing.

<!-- source: internal/plugins/rsvpte/engine.go -- sendResv, the refresh path -->

## Decision: link failure comes from the interface component

Ze has no IGP, so the interface component's netlink down event is the available
link-state source. The engine subscribes to it and matches LSPs by their
admission interface. A transit or egress node sends a PathErr upstream (RFC 3209
error code 24, value 5, "No route available toward destination"); an ingress node
emits a local path-error event. Local state is torn down after either.

<!-- source: internal/plugins/rsvpte/engine.go -- handleLinkDown -->

**Limit.** The match is by admission interface. An LSP whose admission was
skipped, because no interface address prefix resolved, carries an empty
admission interface and is not matched. Precise next-hop to interface resolution
would be needed to close that.

**Concurrency.** Link-failure handling runs on the interface event goroutine, and
the cleanup loop mutates the LSP table off the engine goroutine too. The LSP
table and the admission controller are concurrency-safe for that reason.

## Decision: reload reconciles through OnConfigApply

`OnConfigure` is startup-only. On a reload only `OnConfigVerify` (stash the
pending config) and `OnConfigApply` (commit) fire. Tunnel setup therefore runs
from `reconcileTunnels`, called from both `OnStarted` and `OnConfigApply`.

<!-- source: internal/plugins/rsvpte/register.go -- reconcileTunnels, the OnConfigApply hook -->

Without the `OnConfigApply` caller an added tunnel never signals, a changed
explicit route never reroutes, and a removed tunnel leaks its LSP and its FIB
state. The head-end teardown and reroute paths are unreachable in that state, so
they are also untested by construction.

## Decision: the head-end holds the recorded path

Each node prepends its address to the Record Route Object as the RESV travels
upstream (RFC 3209 section 4.4), so the head-end's reservation state block holds
the full path for `show rsvp-te session`.

<!-- source: internal/plugins/rsvpte/engine.go -- recordRoute -->
<!-- source: internal/plugins/rsvpte/rro.go -- prependRRO, formatERO, formatRRO -->

## Trap: the delivered config shape

The same shape trap as LDP applies: root-wrapped config, numbers as strings, and
YANG lists delivered as **maps keyed by the list key**. Explicit-route hops
arrive as a map keyed by index, so they must be re-sorted by numeric index or the
path order is lost. A parser reading the wrong shape leaves the engine idle.

## Trap: a show proxy must forward, never re-dispatch

`Dispatcher.ForwardToPlugin`, never `Dispatch`. Re-dispatching re-matches the
builtin and recurses.

## Interop: no open-source peer exists

No actively maintained open-source daemon implements RSVP-TE signaling. FRR ships
`ldpd`, not an RSVP daemon; BIRD and GoBGP do BGP-signaled MPLS only.
Cross-vendor interop needs a proprietary lab container.

The substitute is a multi-node Ze-to-Ze harness: two to four real engines wired
through an in-memory fabric, so each engine's own encoded PATH, RESV, PathErr and
PathTear is decoded and acted on by a peer. It covers a direct LSP both ways, a
three-node label stack (push, swap, pop) with RRO across hops, PathErr rejection,
hop-by-hop teardown, soft-state refresh, admission denial, and make-before-break
reroute including the config-reload-triggered form.

Verify a named interop peer exists before an acceptance criterion names it.

## Dataplane

The engine writes swap and pop entries onto the MPLS FIB event bus; the kernel
backend programs them as `AF_MPLS` routes. See
[`../mpls/mpls-kernel.md`](../mpls/mpls-kernel.md).

<!-- source: internal/plugins/rsvpte/fib.go -- busFIB programSwap, programPop, programPush -->
<!-- source: internal/core/mplsfib/events.go -- the MPLS forwarding-entry event -->
