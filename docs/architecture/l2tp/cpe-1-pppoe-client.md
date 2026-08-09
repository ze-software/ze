# PPPoE client interface kind

The client half of PPPoE, for the customer-premises case: dial an access
concentrator over a physical Ethernet interface, negotiate LCP, authentication
and IPCP, and present the resulting PPP session as a routable interface with
server-assigned addresses.

<!-- source: internal/component/iface/pppoe_client.go -- PPPoEClient, PPPoEClientConfig, PPPoEDialer, reconcilePPPoEClients -->
<!-- source: internal/component/l2tp/pppoeclient/dialer.go -- Dialer.Dial, waitForPADO, waitForPADS -->
<!-- source: internal/component/l2tp/pppoeclient/session.go -- negotiateSession, negotiateLCP, negotiateIPCP, keepaliveLoop -->
<!-- source: internal/component/l2tp/pppoeclient/auth.go -- client-mode authentication helpers -->

## RFC obligations carried by this code

RFC 2516 Section 5.3 requires the PADR to echo the Relay-Session-Id tag when one
was present in the PADO. It matters only where a relay agent is in the path, and
it is a MUST regardless.

## Decisions

**A separate package, because of an import cycle.** The pppoe package imports
the iface package for the backend lookup, so iface cannot import pppoe. The
`PPPoEDialer` interface is defined in iface and implemented in the pppoeclient
package, which imports both pppoe for the wire format and ppp for the state
machine and the kernel setup. Registration is an `init()` plus a blank import in
the hub.

**Client-mode PPP drives the FSM directly, it does not extend the PPP driver.**
The existing driver is server oriented: it sends the authentication challenge,
assigns addresses from a pool, and uses external authentication and address
event channels. Client mode reverses all of that. Adding client branches to
every server-side handler would put the L2TP and BNG path at risk, so the client
calls the ppp package's exported pure functions instead.

**One reader goroutine for the whole session lifetime.** The negotiation phase
creates a reader that feeds a frame channel, and the same channel is handed to
the keepalive loop after negotiation. There is no second reader and no
descriptor race.

**Reconciliation follows the DHCP shape.** Desired against active map diffing, a
config-change check that restarts affected clients, and a shutdown loop that
stops all of them.

<!-- source: internal/component/iface/pppoe_client.go -- pppoeClientConfigChanged, ReconnectDelay -->

## Traps this code exists to avoid

**An empty `default:` in a select is refused by a repository hook.** The
non-blocking read shape has to be restructured into a reader goroutine plus a
channel.

**`strconv.FormatInt` is refused in production code.** MAC formatting uses
`net.HardwareAddr.String()`.

**A YANG leaf gated on Linux is pruned on macOS.** The config walker removes the
whole PPPoE client list on a non-Linux host, so `ze config validate` rejects it
as an unknown path. The functional parse test skips that operating system.

## Review findings worth remembering

Every one of these was found by review before the code shipped. They are the
failure shapes a new interface kind repeats:

| Finding | Root cause |
|---------|-----------|
| No shutdown cleanup for the clients | copied the DHCP reconcile and left out the shutdown loop |
| Channel and unit descriptors leaked in the cleanup closure | the PPPoE server closes them through the PPP driver, and the client has no driver |
| Two reader goroutines on the same channel descriptor | negotiation started one and the keepalive loop started another |
| Echo failure fired at four losses, not three | `>` against `>=` with a post-increment |
| A reload did not detect a changed parameter | the DHCP reconcile detects it and this one did not |
| The source interface was not validated | other interface kinds validate their key name |
| The discovery socket read blocked | it needs a receive timeout to stay responsive to the stop signal |
