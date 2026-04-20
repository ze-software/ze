# MCP Remote Access

<!-- source: internal/component/mcp/schema/ze-mcp-conf.yang -- bind-remote, auth-mode, oauth, tls -->
<!-- source: internal/component/config/loader_extract.go -- MCPListenConfig.Validate -->

Ze's MCP server defaults to loopback binding (`127.0.0.1`). Two patterns make
it reachable from other machines:

1. **Tunnel-based** (recommended for dev): keep MCP on loopback, use SSH or
   WireGuard as the encrypted transport. Operator does not have to manage
   TLS certs.
2. **Native remote**: bind MCP to a non-loopback address with authentication
   and TLS. Operator-managed certificate; client-side discovery via RFC 9728.

## Option 1: Tunnel-based (loopback binding)

This is the Phase 1 posture. MCP binds `127.0.0.1`, you tunnel from your
client host. Ze's config verifier accepts `auth-mode none` on loopback, so no
token is required. Add `auth-mode bearer` if the loopback is shared by
untrusted local users (e.g., multi-tenant hosts).

## Starting the MCP Server

```bash
ze start --mcp 8080 bgp.conf
```

This listens on `127.0.0.1:8080`. Only local connections are accepted.

## SSH Port Forwarding

SSH forwards a port on the remote machine's loopback to your local machine (or
vice versa). The traffic is encrypted inside the SSH session.

### Forward a Remote MCP Port to Your Local Machine

You have Ze running on `router.example.com` with `--mcp 8080`. You want to
reach it from your laptop.

```bash
# On your laptop:
ssh -L 8080:127.0.0.1:8080 user@router.example.com
```

This binds `127.0.0.1:8080` on your laptop. Requests to `localhost:8080` travel
through the SSH tunnel and arrive at `127.0.0.1:8080` on the router.

```bash
# Test from your laptop:
curl -s http://localhost:8080/ -X POST \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

### Let a Remote Machine Reach Your Local MCP

You have Ze running locally with `--mcp 8080`. You want a remote machine to
reach it.

```bash
# On your local machine:
ssh -R 8080:127.0.0.1:8080 user@remote.example.com
```

This binds `127.0.0.1:8080` on the remote machine. Programs there can connect
to `localhost:8080` and the traffic tunnels back to your local Ze.

> **Note:** By default, SSH remote forwards bind to `127.0.0.1` on the remote
> side. This is correct -- do not set `GatewayPorts yes` in `sshd_config`, as
> that would expose the port on all interfaces.

### Running in the Background

Add `-f -N` to run the tunnel without an interactive shell:

```bash
ssh -f -N -L 8080:127.0.0.1:8080 user@router.example.com
```

`-f` backgrounds after authentication. `-N` skips shell execution.

To make it persistent across reboots, use `autossh` or a systemd unit:

```ini
# /etc/systemd/system/ze-mcp-tunnel.service
[Unit]
Description=SSH tunnel to Ze MCP
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/bin/ssh -N -L 8080:127.0.0.1:8080 user@router.example.com
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Use key-based authentication (no passphrase, or with `ssh-agent`) for unattended
tunnels.

### Multiple MCP Servers

Forward different remote MCP ports to different local ports:

```bash
ssh -L 8081:127.0.0.1:8080 user@router-a.example.com &
ssh -L 8082:127.0.0.1:8080 user@router-b.example.com &
```

`localhost:8081` reaches router-a, `localhost:8082` reaches router-b.

## WireGuard Tunnel

WireGuard creates a persistent encrypted tunnel between two machines. Each peer
gets an IP address on the tunnel interface. Ze's MCP server still binds to
`127.0.0.1`, so you combine WireGuard with a local port forward using `socat`
or SSH.

### Setup Overview

```
[laptop wg0: 10.0.0.2] ---WireGuard--- [router wg0: 10.0.0.1]
                                           |
                                     127.0.0.1:8080 (ze --mcp 8080)
```

### Router Side (where Ze runs)

```ini
# /etc/wireguard/wg0.conf
[Interface]
Address = 10.0.0.1/24
ListenPort = 51820
PrivateKey = <router-private-key>

[Peer]
PublicKey = <laptop-public-key>
AllowedIPs = 10.0.0.2/32
```

Ze binds MCP to `127.0.0.1:8080`. The WireGuard interface alone does not expose
it -- you need a local relay.

**Option A: socat relay (lightweight)**

```bash
socat TCP-LISTEN:8080,bind=10.0.0.1,fork TCP:127.0.0.1:8080
```

This listens on the WireGuard IP (`10.0.0.1:8080`) and forwards to Ze's
loopback port. Only WireGuard peers can reach `10.0.0.1`.

**Option B: SSH over WireGuard (no relay needed)**

Skip socat entirely. From the laptop, SSH to the router's WireGuard IP and
forward the port:

```bash
ssh -L 8080:127.0.0.1:8080 user@10.0.0.1
```

This is the cleanest approach: WireGuard encrypts the outer transport, SSH
forwards the port, and Ze never leaves loopback.

### Laptop Side

```ini
# /etc/wireguard/wg0.conf
[Interface]
Address = 10.0.0.2/24
PrivateKey = <laptop-private-key>

[Peer]
PublicKey = <router-public-key>
Endpoint = router.example.com:51820
AllowedIPs = 10.0.0.1/32
PersistentKeepalive = 25
```

If using socat on the router:

```bash
curl -s http://10.0.0.1:8080/ -X POST \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

If using SSH over WireGuard:

```bash
curl -s http://localhost:8080/ -X POST \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

### Generating WireGuard Keys

```bash
wg genkey | tee privatekey | wg pubkey > publickey
```

Run on both machines. Exchange public keys. Keep private keys secret.

## Which Approach to Use

| Criteria | SSH Forwarding | WireGuard + socat | WireGuard + SSH |
|----------|---------------|-------------------|-----------------|
| Setup effort | Minimal | Moderate | Moderate |
| Persistent | With systemd/autossh | Yes | With systemd/autossh |
| Multi-service | One tunnel per port | One tunnel, many ports | One tunnel + SSH forward |
| MCP stays on loopback | Yes | No (socat binds wg0 IP) | Yes |
| Extra software | None (SSH is standard) | WireGuard + socat | WireGuard + SSH |

**Recommendation:** SSH forwarding for ad-hoc access. WireGuard + SSH for
permanent infrastructure where you already run WireGuard between sites.

## Option 2: Native remote binding

When tunnelling is impractical (many clients, unattended fleets, OAuth-managed
identities), bind MCP directly to a non-loopback address with TLS and
authentication:

```
environment {
    mcp {
        enabled true;
        bind-remote true;
        auth-mode oauth;
        oauth {
            authorization-server https://auth.example/;
            audience             https://mcp.example/;
            required-scopes      [ mcp.admin ];
        }
        tls {
            cert /etc/ze/mcp.pem;
            key  /etc/ze/mcp.key;
        }
        server public {
            ip 0.0.0.0;
            port 443;
        }
    }
}
```

Alternatively for smaller deployments, use `auth-mode bearer-list` with
per-identity tokens:

```
environment {
    mcp {
        enabled true;
        bind-remote true;
        auth-mode bearer-list;
        identity alice { token <long-random-string>; scope [ mcp.read mcp.write ]; }
        identity bob   { token <long-random-string>; scope [ mcp.read ]; }
        tls {
            cert /etc/ze/mcp.pem;
            key  /etc/ze/mcp.key;
        }
        server public {
            ip 0.0.0.0;
            port 443;
        }
    }
}
```

### Config verify rejects unsafe combinations

`ze config validate` rejects at verify time:

| Configuration | Rejection |
|---------------|-----------|
| `bind-remote true` + `auth-mode none` | `bind-remote requires auth-mode != none` |
| `auth-mode oauth` without `oauth.authorization-server` | `auth-mode=oauth requires oauth.authorization-server` |
| `auth-mode oauth` without `oauth.audience` | `auth-mode=oauth requires oauth.audience` |
| `auth-mode oauth` + non-loopback listener without TLS | `auth-mode=oauth requires tls.cert and tls.key on non-loopback listeners` |
| `auth-mode bearer-list` without any `identity` entries | `auth-mode=bearer-list requires at least one identity` |

These are exact-or-reject gates (`rules/exact-or-reject.md`): a misconfigured
remote endpoint fails the verifier, never silently accepts.

## Security Notes

- With `bind-remote false` (default), every server entry is force-rewritten
  to `127.0.0.1` at config extraction time, preserving the Phase 1 loopback
  clamp even if the operator mistakenly types `0.0.0.0`.
- For tunnel-based deployments, use key-based SSH authentication for
  unattended tunnels. Disable password authentication on routers exposed
  to the internet.
- With WireGuard + socat, only peers in `AllowedIPs` can reach the socat
  listener. This is your access control boundary.
- Rotate WireGuard keys periodically. Revoke a peer by removing its `[Peer]`
  block and reloading (`wg syncconf wg0 <(wg-quick strip wg0)`).
- For native remote deployments, rotate the TLS cert before it expires.
  Hot-reload is not yet supported; restart the daemon after cert rotation.
- OAuth access tokens are never logged. `Authorization` headers are scrubbed
  from debug logs. Token audience is validated on every request (RFC 8707).
- JWKS refresh is rate-limited to 30 s minimum interval; an unknown-kid
  spray cannot trigger a JWKS-fetch flood against the AS.
