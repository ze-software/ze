# Fleet Configuration

Ze supports centralized configuration management for multi-node deployments. A central hub serves configuration to remote ze instances over TLS.
<!-- source: cmd/ze/hub/main.go -- hub orchestrator; internal/component/hub/ -- hub implementation -->

> **Status:** This feature is partially implemented. The hub architecture and blob storage are functional. Client-side config fetching is in design.

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
<!-- source: cmd/ze/init/main.go -- managedFlag, forceFlag -->

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
            host 127.0.0.1;
            port 1790;
            secret "local-plugin-secret";
        }
        server central {
            host 0.0.0.0;
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
            host 127.0.0.1;
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

On the hub, manage client configurations through the config editor:

```bash
ze config edit edge-01.conf         # Edit config for edge-01
ze config archive backup edge-01.conf  # Archive a version
ze config history edge-01.conf      # View rollback history
```
<!-- source: cmd/ze/config/cmd_edit.go; cmd/ze/config/cmd_history.go; cmd/ze/config/cmd_archive.go -->

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
<!-- source: internal/component/hub/ -- hub TLS and client auth -->
