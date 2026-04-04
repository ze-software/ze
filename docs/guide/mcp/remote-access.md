# MCP Remote Access

Ze's MCP server binds exclusively to `127.0.0.1`. It is not possible to bind it
to a public interface -- this is a deliberate security decision. By default the
MCP endpoint has no authentication. To require a bearer token, use `--mcp-token`,
the `ze.mcp.token` env var, or the `token` leaf in the MCP config block (see
[overview.md](overview.md#authentication)).

To access the MCP server from another machine, use an encrypted tunnel. This page
covers two approaches: SSH port forwarding (simple, per-session) and WireGuard
(persistent, site-to-site).

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

## Security Notes

- Never bind the MCP server to `0.0.0.0` or a public IP. The `--mcp` flag
  hardcodes `127.0.0.1` to prevent this.
- For additional security, configure a bearer token (`--mcp-token` or
  `ze.mcp.token` env var). Without a token, anyone who can connect can run
  BGP commands. The tunnel provides transport encryption; the token provides
  authentication.
- Use key-based SSH authentication for unattended tunnels. Disable password
  authentication on routers exposed to the internet.
- With WireGuard + socat, only peers in `AllowedIPs` can reach the socat
  listener. This is your access control boundary.
- Rotate WireGuard keys periodically. Revoke a peer by removing its `[Peer]`
  block and reloading (`wg syncconf wg0 <(wg-quick strip wg0)`).
