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

See [configuration guide](../configuration/index.md#pppoe-access) for all settings.

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
