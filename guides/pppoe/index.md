# PPPoE Access Concentrator

Ze implements an RFC 2516 PPPoE access concentrator for direct-attach
subscriber access. PPPoE is the alternative to L2TP: subscribers connect
over Ethernet to the BNG without an intermediate LAC/LNS tunnel.

## Architecture

PPPoE uses the same transport-agnostic PPP Driver as L2TP. The PPPoE
component handles discovery (PADI/PADO/PADR/PADS/PADT) and creates
kernel PPPoE sessions via AF_PPPOX. The resulting /dev/ppp file
descriptors feed into the PPP Driver, which runs LCP, authentication,
and IPCP/IPv6CP identically to L2TP sessions.

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
    padi-rate-limit 100
    interface eth0 {
    }
    interface eth0.100 {
        service-name "vlan100"
        max-sessions 1000
    }
}
```

See [configuration guide](../configuration-model/index.md#pppoe-access) for all settings.

The subsystem starts only when `enabled` is true and the block lists at least
one `interface`. A `pppoe` block with no `interface` entry registers nothing and
answers no PADI.

### Subscriber authentication

`auth-method` is the PPP Auth-Protocol the access concentrator puts in its own
LCP Configure-Request: `chap-md5` (the default), `pap`, `ms-chap-v2`, or `none`.
`none` requires `allow-no-auth true` beside it, because an access concentrator
that asks nobody who they are is a decision and not a default. That combination,
and an `auth-method` value the PPP driver does not know, are both refused when
the daemon starts, with `parse pppoe config: ...`.

| Leaf | Default | Values | Description |
|------|---------|--------|-------------|
| `auth-method` | `chap-md5` | `none`, `pap`, `chap-md5`, `ms-chap-v2` | Auth-Protocol advertised in the AC's LCP Configure-Request (RFC 1661 Section 6.2) |
| `allow-no-auth` | `false` | boolean | Admit a subscriber whose LCP ends with no Auth-Protocol negotiated |

The default matches the L2TP LNS default, so an operator who configures a
credential for one transport gets the same treatment on the other.

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

The PPP auth handler slot holds one handler, and configuration decides its
owner: `l2tp-auth-radius` claims it when a RADIUS server is configured, and
`l2tp-auth-local` keeps it otherwise. A box with local users and no RADIUS block
therefore authenticates against those users, on PPPoE and on L2TP.
<!-- source: internal/component/l2tp/plugins/authradius/register.go -- activateRadiusConfig claims the slot -->

## Interoperability

Two Docker lab scenarios run Ze in each PPPoE role. `make
ze-deployment-docker-pppoe-accel-test` runs both.

| Scenario | Ze role | Peer | Proves |
|----------|---------|------|--------|
| `01-pppoe-chap-ipv4` | client | accel-ppp | Discovery, LCP, CHAP-MD5, the IPCP address on a kernel `pppN` interface, a ping to the AC gateway through the session, and a clean teardown |
| `02-ze-ac-pppd-client` | access concentrator | pppd 2.5.1 with the rp-pppoe plugin | Discovery, LCP demanding CHAP-MD5, the credential accepted, an IPCP pool address, ICMP across the session, and a wrong password refused before IPCP |

`make ze-qemu-pppoe-accel-test` runs the client half in QEMU, and `make
ze-qemu-pppoe-test` runs the access concentrator's own `test/pppoe/` suite on
Ze's runtime kernel (stock Alpine has no `CONFIG_PPPOE`).

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
- **Service-Name filtering**: Only PADIs matching configured service names
  are accepted. Empty list means accept any.
- **MAC binding**: Sessions are bound to the subscriber MAC from PADR.
  PADTs from other MACs are rejected.

## Concurrent Operation

PPPoE and L2TP run concurrently on the same daemon. Both share the same
PPP Driver, auth handlers, IP pools, and shaper plugins. The PPP
component distinguishes sessions by TunnelID (ifindex for PPPoE, tunnel
ID for L2TP) and SessionID.

<!-- source: internal/component/l2tp/pppoe/subsystem.go -->
<!-- source: internal/component/l2tp/pppoe/server.go -->
<!-- source: internal/component/l2tp/pppoe/discovery.go -->
