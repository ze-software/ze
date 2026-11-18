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
            secret "local-plugin-secret-of-32-characters";
        }
        server central {
            ip 0.0.0.0;
            port 1791;
            secret "remote-plugin-secret-of-32-character";
            client edge-01 { secret "edge-01-secret-of-at-least-32-chars"; }
            client edge-02 { secret "edge-02-secret-of-at-least-32-chars"; }
        }
    }
}
```

Each client authenticates with its own secret, which is bound to its name: one
client's secret is refused under another client's name. Every secret is at
least 32 characters, and a shorter one is refused when the config loads.

A client is admitted only on the address of the `server` block that declares
it. With two blocks that accept managed clients, a client declared under one is
refused on the other's address. Declare it under both if it must reach both.

The block that accepts managed clients needs its own port. The first `server`
block is the one local plugins connect to, and one address cannot serve both.
<!-- source: internal/component/plugin/server/managed_serve.go -- ManagedListener, handleConn -->

## Client Configuration

A managed client specifies its hub connection:

```
plugin {
    hub {
        server local {
            ip 127.0.0.1;
            port 1790;
            secret "local-secret-of-at-least-32-characters";
        }
        client edge-01 {
            host 10.0.0.1;
            port 1791;
            secret "edge-01-secret-of-at-least-32-chars";
            ca fleet-hub-root;
        }
    }
}
```

The hub pushes this block with the rest of the config. When a pushed config
changes it (another hub address, a new secret, another `ca`), the running
client reconnects with the new values as soon as it has applied the config. No
restart is needed. A pushed config with no `client` block makes the client stop
following its hub.
<!-- source: internal/component/managed/client.go -- adoptEndpoint -->

## Trusting the Hub

The client authenticates the hub before it sends its secret. Unless the hub's
certificate comes from a public certificate authority, the client must hold the
root that issued it. By default the hub serves a certificate from its own
authority, so:

1. On the hub, print the root: `ze show pki local-ca pem`
2. In that client's config (the one the hub stores for it), paste it into a
   `pki ca` block and name that block from the client's `ca` leaf:

```
pki {
    ca fleet-hub-root {
        certificate "-----BEGIN CERTIFICATE-----
MIIB...
-----END CERTIFICATE-----";
    }
}
```

The root outlives a hub restart, so the client needs no change when the hub
renews its certificate.

A first start has no config yet, so it has no `ca` to name. It verifies the hub
against the system's certificate authorities. Against a hub that serves a
certificate from its own authority, that first start fails unless
`ze.managed.tls.insecure` is set, which sends the secret to a hub the client
has not authenticated. Use it only on a network you trust, and only for the
first start: every connection after the first fetch is verified against `ca`.
<!-- source: internal/component/managed/tls.go -- clientTLSConfig -->
<!-- source: cmd/ze/ze_core_start.go -- fetchInitialConfig -->

## First Start

`ze init --managed` marks the instance as managed. It does not record where the
hub is. The first start names the hub, the client and its secret:

```bash
ze --server 10.0.0.1:1791 --name edge-01 --token "edge-01-secret-of-at-least-32-chars" start
```

The client fetches its config, checks it, caches it and starts. Every later
start is a plain `ze start`: the cached config carries the `client` block.
Without a cached config and without `--server` and `--name`, `ze start` exits
with an error that names what is missing.
<!-- source: cmd/ze/ze_core_start.go -- cmdStartManaged -->

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
<!-- source: internal/component/config/storage/cli/data_rpc.go -- restoreConfigFromSource -->
<!-- source: internal/component/config/cli/cmd_edit.go -- config edit command -->
<!-- source: internal/component/config/cli/cmd_history.go -- config history command -->
<!-- source: internal/component/config/cli/cmd_archive.go -- config archive command -->

## When a Client Cannot Connect

The hub admits only the names it has a `client` entry for, each with its own
secret. A client that connects under any other name is refused whatever token
it sends, and so is a configured name with the wrong secret. `show doctor` on
the hub lists each refusal under `doctor-managed-client-rejected` with the
name the client claimed and the fix; it never shows the token. The same check
reports a second ze connecting under a name already connected, and a client
that refused to apply the config the hub serves it.

`show doctor` also reports a managed listen address the hub could not bind
(`doctor-managed-listener-unbound`), and a `config-changed` a connected client
was not given (`doctor-managed-client-notify-dropped`). The hub disconnects a
client that does not accept the notice, so that it reconnects and fetches: a
client fetches its config each time it connects. Until then it runs its
previous config.
<!-- source: internal/component/plugin/doctor/check_managed_runtime.go -- checkManagedClientRejected, managedRejectedRow, checkManagedListenerUnbound -->
<!-- source: internal/component/plugin/server/managed_doctor.go -- authRefused, notifyDropped -->

## Resilience

- **Partition tolerance:** Clients cache their last known config locally
- **Fallback:** If the hub is unreachable, the client starts with cached config
- **Reconnect:** Clients automatically reconnect and receive updated config

## CLI Overrides

The flags go before the command: `ze --server 10.0.0.1:1791 start`.

| Flag | Environment | Description |
|------|-------------|-------------|
| `--server <host:port>` | `ze.managed.server` | Hub address |
| `--name <name>` | `ze.managed.name` | Client name |
| `--token <secret>` | `ze.managed.token` | Client secret |
| `--connect-timeout <duration>` | `ze.managed.connect.timeout` | Bound on reaching the hub, per attempt (default `5s`) |
| (none) | `ze.managed.tls.insecure` | Skip verification of the hub's certificate. See "Trusting the Hub" |

`--server`, `--name` and `--token` name the hub on a first start. On a client
that holds a cached config they replace the matching value of its `client`
block for as long as the daemon runs, which is for troubleshooting.
<!-- source: cmd/ze/ze_core_dispatch.go -- zeParseGlobalFlags -->
<!-- source: cmd/ze/ze_core_start.go -- hubClientEndpoint -->
