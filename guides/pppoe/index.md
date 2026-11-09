# PPPoE Access Concentrator

Ze implements an RFC 2516 PPPoE access concentrator for direct-attach
subscriber access. PPPoE is the alternative to L2TP: subscribers connect
over Ethernet to the BNG without an intermediate LAC/LNS tunnel.

## Architecture

PPPoE uses the same transport-agnostic PPP Driver as L2TP. The PPPoE
component handles discovery (PADI/PADO/PADR/PADS/PADT) and creates
kernel PPPoE sessions via AF_PPPOX. The resulting /dev/ppp file
descriptors feed into the PPP Driver, which runs LCP, authentication,
and IPCP/IPv6CP through the shared state machines.

PPPoE applies RFC 2516's transport restrictions to LCP: neither role requests
ACCM, ACFC or FCS Alternatives, and both reject peer requests for them.
These restrictions do not change L2TP negotiation. A PADT matching the
session ID and both MAC addresses ends the PPPoE transport immediately;
PPP termination packets are not sent afterwards. Local teardown also
stops PPP before sending PADT. A subscriber's LCP Terminate-Request is
answered with a Terminate-Ack, and one Restart time later (3 seconds) the
AC sends the PADT, with User Request as the accounting cause; a new LCP
Configure-Request arriving before the PADT completes is still answered.

```
Subscriber CPE
    |
    | Ethernet (ethertype 0x8863 discovery, 0x8864 session)
    v
PPPoE Subsystem (internal/component/l2tp/pppoe/)
    |
    | StartSession{ChanFD, UnitFD, AccessInterface, SubscriberMAC, ...}
    v
PPP Driver (internal/component/l2tp/ppp/)
    |
    v
Auth/Pool/Shaper plugins (shared with L2TP)
```

## Configuration

```
pppoe {
    enabled true
    ac-name "my-bng"
    service-name "internet"
    auth-method chap-md5
    cookie-timeout 5
    max-sessions 65535
    max-sessions-per-mac 8
    padi-rate-limit 100
    interface eth0 {
    }
    interface eth0.100 {
        service-name "vlan100"
        max-sessions 1000
        max-sessions-per-mac 2
    }
}
```

See [configuration guide](../configuration-model/index.md#pppoe-access) for all settings.

Discovery runs only when `enabled` is true and the block lists at least one
`interface`. A `pppoe` block with no `interface` entry answers no PADI.

### Changing the configuration

A committed change to the `pppoe` block applies to the running access
concentrator at once, with no restart:

| Change | Effect |
|--------|--------|
| `ac-name`, `service-name`, `cookie-timeout`, `padi-rate-limit`, `max-sessions`, `max-sessions-per-mac`, `auth-method`, `authentication-allow`, `allow-no-auth` | Apply to the next PADI and the next admitted session. A session already up keeps the terms it was admitted under. A lowered `max-sessions` refuses new sessions and ends none. AC-Cookies already handed out stay valid |
| An added `interface` | Served at once. An interface that does not exist refuses the commit, naming it and the `delete pppoe interface <name>` that undoes it |
| A removed `interface` | Stops answering PADI. Each of its sessions gets a PADT and its PPP session is stopped |
| `enabled true`, or a first `interface` | Starts discovery, even when the daemon booted with PPPoE disabled |
| `enabled false`, or no `interface` left | Ends every session with a PADT and stops discovery |

### Subscriber authentication

`auth-method` is the PPP Auth-Protocol the access concentrator puts in its own
LCP Configure-Request: `chap-md5` (the default), `pap`, `ms-chap-v2`, or `none`.
With `pap` and its derived default policy, the first Configure-Request offers
CHAP-MD5, and PAP is offered only after the subscriber sends a Configure-Nak
that suggests PAP (RFC 1334 Section 2: an implementation that includes CHAP
MUST offer it before PAP). An explicit `[pap]` policy offers PAP directly.
<!-- source: internal/component/l2tp/ppp/auth.go -- initialAuthMethod -->
`none` requires `allow-no-auth true`. The daemon rejects `none` without that
opt-in, and rejects an unknown authentication method, with
`parse pppoe config: ...`.

| Leaf | Default | Values | Description |
|------|---------|--------|-------------|
| `auth-method` | `chap-md5` | `none`, `pap`, `chap-md5`, `ms-chap-v2` | Auth-Protocol advertised in the AC's LCP Configure-Request (RFC 1661 Section 6.2) |
| `authentication-allow` | Derived from `auth-method` | ordered set of `pap`, `chap-md5`, `ms-chap-v2` | Permitted methods; must include `auth-method`, cannot include `none` or accompany `auth-method none` |
| `allow-no-auth` | `false` | boolean | Admit a subscriber whose LCP ends with no Auth-Protocol negotiated |

The default matches the L2TP LNS default, so an operator who configures a
credential for one transport gets the same treatment on the other.
Explicit no-auth sessions need no local user entry.

The flat `authentication-allow` leaf sits beside `auth-method`:

```
pppoe {
    auth-method chap-md5
    authentication-allow [chap-md5 ms-chap-v2 pap]
}
```

Absent or empty lists derive `[chap-md5]` for `chap-md5`,
`[ms-chap-v2]` for `ms-chap-v2`, `[chap-md5 pap]` for `pap`, and no
methods for `none`. Lists are ordered sets: duplicate entries do not add a
second attempt. The configured CHAP variant is offered first; remaining CHAP
variants follow written order, before PAP regardless of its written position.
Changes affect newly admitted sessions only, not an existing session's policy.

A Configure-Nak selects an untried permitted method; an unlisted, unknown or
already-tried suggestion advances through the remaining methods without
repeating an offer. A PAP suggestion cannot skip an untried CHAP variant.
A valid Configure-Reject removes Auth-Protocol for the whole negotiation,
rather than trying a different method. Required authentication ends LCP with
Terminate-Request before Opened when no acceptable method remains or the
option is rejected, followed by PADT when termination completes. No PAP
exchange or auth-handler request occurs on that refusal. Explicit
`allow-no-auth true` instead permits the no-wire-auth handler after the option
is dropped.
<!-- source: internal/component/l2tp/ppp/auth.go -- selectAuthFallback, refuseAuthMethod -->

The editor accepts one method per command. To permit PAP explicitly while
retaining CHAP-MD5, use `set pppoe authentication-allow chap-md5`, followed by
`set pppoe authentication-allow pap`. Refusal messages print only PPPoE repair
commands for a PPPoE subscriber. To repair an invalid explicit list by restoring
the method-derived default, use `delete pppoe authentication-allow`.
<!-- source: internal/component/l2tp/ppp/auth.go -- refuseAuthMethod -->
<!-- source: internal/component/l2tp/pppoe/config.go -- ExtractParameters -->
<!-- source: internal/component/cli/model_commands_edit.go -- cmdSet -->
<!-- source: internal/component/cli/editor_commands.go -- SetValue, setLeafListMember -->

**Migration:** CHAP-MD5 and MS-CHAPv2 no longer implicitly permit other
methods. If subscribers previously relied on PAP fallback, explicitly include
both the configured method and `pap`. Do not enable `allow-no-auth` as a
substitute for permitting the authentication method those subscribers need.
PPPoE's MRU ceiling remains fixed at 1492; there is no PPPoE MRU setting.
The installed IP interface MTU is at most 1492, including when the peer
omits its MRU option and thus advertises the PPP default of 1500. A smaller
negotiated peer MRU still lowers the transmit MTU. The kernel receive buffer
keeps its required 1500-octet capacity.
<!-- source: internal/component/l2tp/ppp/session_run.go -- afterLCPOpen -->

The credential comes from the same auth plugins the L2TP LNS uses. Configure a
local user, or a RADIUS server:

```
l2tp {
    auth {
        local {
            user alice {
                password "s3cr3t"
            }
        }
    }
    pool {
        ipv4 {
            gateway 10.20.0.1
            start 10.20.0.2
            end 10.20.0.254
        }
    }
}
```

The `l2tp` block here configures the shared BNG plugins, not an L2TP listener:
`l2tp-auth-local` verifies the credential and `l2tp-pool` supplies the IPCP
address. A PPPoE-only BNG needs neither `l2tp enabled` nor an L2TP server.
The IPCP DNS a PPPoE subscriber gets follows the pool's `dns-source` list, as
on L2TP: with none written, RADIUS's MS DNS servers, then the pool's
`dns-primary` and `dns-secondary`, then the resolvers Ze learned
(`docs/guide/l2tp.md`, "IP address pool").

The PPP auth handler slot holds one handler, and configuration decides its
owner: `l2tp-auth-radius` claims it when a RADIUS server is configured, and
`l2tp-auth-local` keeps it otherwise. A box with local users and no RADIUS block
therefore authenticates against those users, on PPPoE and on L2TP.
<!-- source: internal/component/l2tp/plugins/authradius/register.go -- activateRadiusConfig claims the slot -->

### PPPoE client keepalive and redial

A `pppoe-client` entry detects a dead access concentrator with LCP
Echo-Requests. `keepalive interval` sets the seconds between two requests
(default 10), and `keepalive failures` sets how many unanswered requests in a
row end the session (default 3), so the default detection time is about 30
seconds. These play the part of pppd's `lcp-echo-interval` and
`lcp-echo-failure`, with one difference in counting: Ze ends the session when
it sends the request that reaches `failures`, while pppd ends it at the next
tick after its Nth unanswered request. pppd's `lcp-echo-failure 3` therefore
matches `keepalive failures 4`. After a session ends or a dial fails, the client waits
before it dials again: the first wait is `reconnect delay-min` (default 1
second), and each further failure doubles it up to `reconnect delay-max`
(default 60 seconds). A change to any of these leaves keeps the running
session up: the next session and the next wait use the new values. Every leaf
and its range is listed in [Configuration](../configuration-model/index.md), "PPPoE Client".
<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- pppoe-client keepalive, reconnect -->
<!-- source: internal/component/l2tp/pppoeclient/network_phase.go -- keepaliveLoop -->
The `access-reconnect` topology scenario proves the redial: after the LNS
drops the session and RADIUS hands out a new address, the CPE comes back with
it, the IKE SA follows it and masquerade uses it.
<!-- source: internal/le/interoplab/topology/check_vpn.go -- checkAccessReconnect -->

### PPPoE client IPv6

`ipv6 enabled true` on a `pppoe-client` negotiates IPv6CP beside IPCP. The
client installs the link-local address formed from the negotiated interface
identifier on the session device. An access concentrator that rejects or
ignores IPv6CP leaves the IPv4 session up. The same container takes
`accept-ra`, `autoconf` and `forwarding`, written to the session device each
time a session creates it, and a `dhcpv6` block that runs the DHCPv6 client on
the session device. Ze refuses the other `ipv6` leaves while `enabled` is not
true. For a router that learns its default route from the access
concentrator's Router Advertisements and asks for a delegated /56:

```
pppoe-client pppoe0 {
    source-interface eth2
    authentication { username user@isp.example; password secret; }
    ipv6 {
        enabled true
        accept-ra 2
        autoconf true
        forwarding true
        dhcpv6 { enabled true; pd { length 56; } }
    }
}
```

A PPP device has no hardware address, so the DHCPv6 client builds its DUID-LL
and IAIDs from the source interface unless `dhcpv6 duid` is set. The delegated
prefix is published as a lease on `pppoe0`.

A LAN unit takes a /64 from it with `ipv6 delegated-prefix`, named after the
`pppoe-client`:

```
interface ethernet eth1 {
    unit 0 {
        ipv6 {
            forwarding true
            delegated-prefix pppoe0 { subnet-id 1; interface-identifier ::1; }
            router-advertisement { enabled true; }
        }
    }
}
```

With `2001:db8:aa00::/56` delegated, `eth1` holds `2001:db8:aa00:1::1/64`. The
subnet id sits in the bits between the delegated length and 64, so a /56 holds
ids 0 to 255; `interface-identifier` defaults to `::1`. The address takes the
lifetimes that remain of the delegation, and a renew refreshes them (RFC 9915
Section 6.3). When the lease ends the address is removed. Config verify refuses a
`subnet-id` the `pd length` cannot hold, two units taking the same id from one
source, and a source that is not a `pppoe-client` asking for a prefix. A server
may delegate a longer prefix than the hint; when it is too small for the id, Ze
logs an error naming the unit and the fix, and the unit gets no /64.

With `router-advertisement` enabled, the unit advertises the /64 for SLAAC and a
Route Information option for the whole delegated prefix, both bounded by what
remains of the delegation. Its Router Lifetime is the configured one only while
`pppoe0` holds an IPv6 default route and the unit has a prefix, and an
advertisement with Router Lifetime 0 leaves at once when either is lost (RFC 7084
G-4, G-5, L-4). While a LAN unit holds space from the prefix, the client never
releases it.

`show interface pppoe-client` lists, under `delegated-prefixes`, each prefix the
client's lease holds with its `valid-lifetime` and `preferred-lifetime`: the
whole seconds that remain when you ask, 4294967295 meaning infinite.
`show interface delegated-prefix` answers one row per `delegated-prefix` entry
of an enabled unit, naming the `interface`, `unit`, `source` and `subnet-id`.
A unit whose `state` is `installed` also carries its `address`, the derived /64
as `prefix`, the whole `delegated` prefix and the lifetimes that remain. The
other states carry a `reason`: `waiting` (the source holds no delegated prefix),
`too-small` (the delegated prefix cannot hold the subnet id, with the fix) and
`not-installed` (the address install failed, and the log names why).

The interop scenarios `pppoe-ipv6cp-ze-client` and
`pppoe-ipv6cp-rejected-ze-client` prove this against accel-ppp: the first the
negotiated link-local address, the RA default route and SLAAC, the second an
access concentrator that rejects IPv6CP (`docs/labs/pppoe-interop.md`).
<!-- source: internal/component/iface/pppoe_client_ipv6.go -- parsePPPoEClientIPv6, applyPPPoEClientSysctls -->
<!-- source: internal/component/iface/register.go -- reconcileDHCP, pppoeClientDHCPUnit -->
<!-- source: internal/component/iface/delegated_consumer.go -- runDelegationWorker, installDelegatedAddress -->
<!-- source: internal/component/iface/delegated_prefix.go -- deriveDelegatedAddress, verifyDelegatedPrefixes -->
<!-- source: internal/component/iface/delegated_lease.go -- DelegatedPrefixStatus, statusOf -->
<!-- source: internal/component/iface/delegated_prefix_command.go -- delegatedUnitRowOf, delegatedUnitStatusOf -->
<!-- source: internal/plugins/iface/ra/advertiser.go -- build, routerLifetime -->
<!-- source: internal/plugins/iface/dhcp/dhcp_v6_linux.go -- releasableLease -->

## Interoperability

Docker lab scenarios run Ze in each PPPoE role; the table names the two
baseline ones and the CPE reconnect, and `docs/labs/pppoe-interop.md` lists
every scenario.
`./le test deployment docker-pppoe-accel` runs them all.

| Scenario | Ze role | Peer | Proves |
|----------|---------|------|--------|
| `01-pppoe-chap-ipv4` | client | accel-ppp | Discovery, LCP, CHAP-MD5, the IPCP address on the kernel device `pppoe0`, named after the `pppoe-client` key, a ping to the AC gateway through the session, and a clean teardown |
| `pppoe-named-device-reconnect-ze-client` | client | accel-ppp | A home CPE: the device is `pppoe0` and the client creates no `pppN`; a static default `next { interface pppoe0 }` reaches the kernel through it; LAN traffic masqueraded on `output-interface "pppoe0"` crosses it and the nft counter rises; after accel-ppp terminates the session, a new `pppoe0` comes up and the route and NAT work again with no reload |
| `02-ze-ac-pppd-client` | access concentrator | pppd 2.5.5 with the rp-pppoe plugin | Discovery, LCP demanding CHAP-MD5, the credential accepted, an IPCP pool address, ICMP across the session, and a wrong password refused before IPCP |

`./le test qemu pppoe-accel` runs the client half in QEMU, and
`./le test qemu pppoe` runs the access concentrator's `test/pppoe/` suite on
Ze's runtime kernel. The stock Alpine kernel has no `CONFIG_PPPOE`.

## CLI Commands

| Command | Description |
|---------|-------------|
| `show pppoe` | Subsystem summary |
| `show pppoe sessions` | List active sessions |
| `show pppoe session <sid>` | Show one session |
| `show pppoe statistics` | Per-interface counters |
| `show pppoe interfaces` | Configured access interfaces |

## Security

- **AC-Cookie**: HMAC-SHA256 cookie in PADO/PADR prevents PADR flooding.
  Cookies expire after `cookie-timeout` seconds (default 5).
- **PADI rate limiting**: Per-source-MAC rate limit prevents discovery
  flooding. Configurable via `padi-rate-limit` (default 100/s).
- **Per-MAC session cap**: A MAC address holding `max-sessions-per-mac`
  sessions is refused a further PADR (PADS error, no allocation).
  Configurable via `max-sessions-per-mac` (default 8), global and
  per-interface. The AC-Cookie proves a MAC completed a PADI/PADO round trip,
  not that a given PADR is fresh, so this cap is the actual bound on how many
  sessions a replayed or forged PADR can make the AC allocate.
- **Service-Name filtering**: Only PADIs matching configured service names
  are accepted. Empty list means accept any.
- **MAC binding**: Sessions are bound to the subscriber MAC from PADR.
  PADTs from other MACs are rejected.

## Metrics

- `ze_pppoe_discovery_refusals_total` -- discovery packets Ze refuses,
  labelled by `reason`. A PADI refused for an unoffered service name gives
  no reply on the wire (RFC 2516 Section 5.2), so this counter is the only
  way to see that refusal without a packet capture.

| Reason | Meaning |
|--------|---------|
| `rate-limited` | The PADI rate limiter dropped a packet from this source MAC |
| `service-name-mismatch` | The requested service does not match any configured `service-name` |
| `service-name-missing` | A PADR carried no Service-Name tag (RFC 2516 Section 5.3) |
| `cookie-invalid` | A PADR's AC-Cookie was missing, malformed, or expired |
| `session-id-exhausted` | The interface's usable session ID space (1 to 65534) is full; 0 and 65535 are reserved |
| `per-mac-cap-reached` | The PADR's source MAC already holds `max-sessions-per-mac` sessions |

- `ze_ppp_ipv6cp_identifier_refusals_total` -- IPv6CP negotiations refused for
  want of a usable interface identifier, labelled by `reason`. Emitted by the
  shared PPP driver, so it counts L2TP and PPPoE subscribers alike; the three
  reasons are listed under "PPP negotiation counters" in `l2tp.md`.

- `ze_pppoe_discovery_read_errors_total` -- discovery socket read errors the
  discovery reader swallowed and retried, after ruling out the socket being
  closed. A separate counter from the refusal series above: a refusal is a
  decision about a well-formed read, this one is the socket itself failing
  to read at all. One AF_PACKET socket serves every configured access
  interface, so this counter is not split by interface.

- `ze_ppp_reader_errors_total` -- socket read errors a PPP reader goroutine
  swallowed and retried, labelled `loop` (`ra`, `dhcpv6`). Emitted by the
  shared PPP driver, so it counts L2TP and PPPoE subscribers alike; listed
  under "Reader-loop error counters" in `l2tp.md`.

## Concurrent Operation

PPPoE and L2TP run concurrently on the same daemon. Both share the same
PPP Driver, auth handlers, IP pools, and shaper plugins. The PPP
component distinguishes sessions by TunnelID (ifindex for PPPoE, tunnel
ID for L2TP) and SessionID.

<!-- source: internal/component/l2tp/pppoe/subsystem.go -->
<!-- source: internal/component/l2tp/pppoe/server.go -->
<!-- source: internal/component/l2tp/pppoe/discovery.go -->
<!-- source: internal/component/l2tp/pppoe/metrics.go -->
<!-- source: internal/component/l2tp/ppp/metrics.go -- initPPPMetrics, countIdentifierRefusal -->
