# The unified subscriber session model

One shared session type, one event namespace and transport-generic handler
registries, used by both L2TP and PPPoE. Before it, three session models existed
side by side, PPPoE emitted no bus events, and authentication, pools, shaping,
accounting and change-of-authorization were wired to L2TP alone. PPPoE sessions
were invisible to downstream plugins and to the show commands.

<!-- source: internal/component/l2tp/subscriber/session.go -- Session, AccessType -->
<!-- source: internal/component/l2tp/subscriber/registry.go -- Registry, Add, Remove, All, ByAccessType, LookupByAcctSessionID -->
<!-- source: internal/component/l2tp/subscriber/handler_registry.go -- RegisterAuthHandler, RegisterPoolHandler, RegisterShaperHandler -->
<!-- source: internal/component/l2tp/subscriber/events/events.go -- the subscriber event namespace -->
<!-- source: internal/component/l2tp/subscriber_bridge.go -- subscriberBridge, onSessionUp, onSessionDown, onSessionIPAssigned -->

## Decisions

**A bridge subscribes to L2TP events and re-emits them as subscriber events.**
Modifying the L2TP reactor directly was rejected: the reactor is heavily tested
and the bridge is a clean seam.

**The internal registry map holds `*Session`, the public API returns copies.**
The session struct is 408 bytes, so range copies were flagged. Callers still get
value copies, so no caller can mutate registry state.

**L2TP handler registration delegates to the subscriber registry.** The L2TP
register functions wrap and forward. Existing L2TP plugins keep working with no
code change, and they do not import the subscriber package.

**Prefix handlers stay L2TP-scoped.** The prefix request and result types carry
a tunnel-id and session-id tuple that is specific to L2TP. Generalization waits
until PPPoE needs DHCPv6 prefix delegation.

**`DefaultRegistry` is a package-level singleton.** L2TP and PPPoE both write to
it and the show commands need one query point.

**A lifecycle consumer subscribes to the subscriber topic alone, never to both
topics.** The address pool was the first to move. It reads
`subevents.SessionDown` and no longer reads `l2tpevents.SessionDown`, because
the bridge re-emits every L2TP teardown onto the subscriber topic. Holding both
subscriptions would deliver each L2TP teardown twice. The two emitters cannot
disagree about whether the bridge exists: `Subsystem.Start` gives the reactor
and the bridge the same event bus, and builds neither when that bus is nil.

**`Session.PPPKey` names the PPP driver's identifier pair.** A consumer that
stored per-session state when the driver asked it for an address reads that
state back under the same pair. It is the tunnel and session id for L2TP, and
the access interface index with the PPPoE session id for PPPoE. The branch lives
on the session rather than in each consumer.

## Consequences worth knowing

- A new access type such as IPoE follows the same three steps: populate the
  default registry, emit subscriber events, register the authentication and pool
  handlers.
- Change-of-authorization can match a session by accounting session id across
  access types.
- `show subscriber summary` is the single view over every session type. The
  access-type commands remain for transport-level detail.

<!-- source: internal/component/l2tp/subscriber/cmd/subscriber.go -- subscriber CLI handlers -->
<!-- source: internal/component/l2tp/subscriber/metrics.go -- session telemetry -->
<!-- source: internal/component/l2tp/subscriber/service.go -- service locator -->

## Traps this code exists to avoid

**Registry insertion must precede the event emit.** The PPPoE event consumer
first handled only session-down. Expanding it to session-up and IP-assigned
required that ordering, or a subscriber to the event does not see the session.

<!-- source: internal/component/l2tp/pppoe/subsystem.go -- handlePPPEvent, onSessionUp, onSessionDown -->
<!-- source: internal/component/l2tp/pppoe/drain.go -- startPPPoEAuthDrain, startPPPoEPoolDrain -->

**A teardown is published even when the registry holds no session.** A session
that fails an NCP, or whose peer disconnects between IPCP and session-up, never
reaches the registry. It does hold the address IPCP allocated for it, so a
publication gated on a registry hit leaks that address. Both transports fall
back to a session built from the identifiers in hand.

<!-- source: internal/component/l2tp/subscriber/session.go -- PPPKey, AccessIfIndex -->
<!-- source: internal/component/l2tp/plugins/pool/register.go -- setEventBus, onSessionDown -->

**The bridge must not be built on a nil event bus.** L2TP subsystem tests start
with a nil bus, and the bridge panics on one. Guard before constructing it.

**Two structurally identical types are still two types.** The auth respond
function and the auth result exist in both the l2tp and the subscriber package
with the same shape and different names. The delegation layer converts field by
field; assignment does not compile and a type conversion would hide a future
divergence.
