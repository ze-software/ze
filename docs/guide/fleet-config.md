# Fleet Configuration

Ze supports centralized configuration management for multi-node deployments. A central hub serves configuration to remote ze instances over TLS.
<!-- source: cmd/ze/hub/managed_server.go -- fleet hub server; internal/component/managed/ -- managed client and fleet protocol -->

> **Status:** Implemented. Named hub blocks, per-client auth, managed client lifecycle (fetch, reconnect, heartbeat), and `ze start` managed mode are functional.

## Architecture

```
    Central Hub                        Remote Nodes
    ┌──────────┐                  ┌──────────┐
    │  ze hub  │ ◄── TLS ────── │ ze edge-01│
    │          │                  └──────────┘
    │ configs/ │ ◄── TLS ────── ┌──────────┐
    │ edge-01  │                  │ ze edge-02│
    │ edge-02  │                  └──────────┘
    └──────────┘
```

The hub stores per-client configurations. Each remote node connects, authenticates, and receives its configuration.

## Bootstrap

Initialize a new ze instance:

```bash
ze init                             # Interactive setup
ze init --managed                   # Managed client (connects to hub)
ze init --force                     # Replace existing database
```
<!-- source: internal/plugins/init/main.go -- managedFlag, forceFlag -->

`ze init` prompts for:
- SSH username and password
- Host and port for the SSH server
- Instance name (e.g., `edge-01`)

### Non-Interactive

```bash
echo -e "admin\nsecret\n10.0.0.1\n2222\nedge-01" | ze init --managed
```

## Hub Configuration

The hub declares which clients can connect:

```
plugin {
    hub {
        server local {
            ip 127.0.0.1;
            port 1790;
            secret "local-plugin-secret";
        }
        server central {
            ip 0.0.0.0;
            port 1791;
            secret "remote-plugin-secret";
            client edge-01 { secret "client-1-token"; }
            client edge-02 { secret "client-2-token"; }
        }
    }
}
```

Each client authenticates with its unique secret token.

## Client Configuration

A managed client specifies its hub connection:

```
plugin {
    hub {
        server local {
            ip 127.0.0.1;
            port 1790;
            secret "local-secret";
        }
        client edge-01 {
            host 10.0.0.1;
            port 1791;
            secret "client-1-token";
        }
    }
}
```

## Config Management

On the hub, manage client configurations through the config editor. Stop the hub daemon
first. Each of these commands opens the `database/` store in its own process. A running hub
answers every read from the tree it loaded at startup. An edit made behind it therefore
changes nothing the hub serves.

```bash
ze config edit client-edge-01.conf         # Edit config for edge-01
ze config archive backup client-edge-01.conf  # Archive to named destination
ze config history client-edge-01.conf      # View rollback history
```

The blob key for a client carries a `client-` prefix. That prefix keeps a client's config
from colliding with the hub's own config file.

A running hub restores a client's config from a backup artifact on its own host, with no
stop. The config becomes a new version of `client-<name>.conf`, the hub pushes
`config-changed` to that client, and the client fetches and applies it. The hub's own
config does not change and the hub does not reload.

```
request data restore path /var/backups/edge-01.zefs config client edge-01
request data restore path /var/backups/fleet.zefs config name edge-01.conf client edge-01
```

The hub refuses `client <name>` when it has no `client <name>` entry under
`plugin hub server`.

<!-- source: internal/component/plugin/server/managed_serve.go -- ClientConfigKey, Serves -->
<!-- source: internal/component/config/storage/cli/data_rpc.go -- restoreClientConfig -->
<!-- source: internal/component/config/cli/cmd_edit.go -- config edit command -->
<!-- source: internal/component/config/cli/cmd_history.go -- config history command -->
<!-- source: internal/component/config/cli/cmd_archive.go -- config archive command -->

## Resilience

- **Partition tolerance:** Clients cache their last known config locally
- **Fallback:** If the hub is unreachable, the client starts with cached config
- **Reconnect:** Clients automatically reconnect and receive updated config

## CLI Overrides

| Flag | Description |
|------|-------------|
| `--server <addr>` | Override hub address |
| `--name <name>` | Override instance name |
| `--token <secret>` | Override auth token |
<!-- source: internal/component/managed/ -- fleet TLS and client auth -->
