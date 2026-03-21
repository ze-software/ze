# Managed Configuration

**Status:** Design (not yet implemented)

**Purpose:** Document the architecture for centralized configuration where ze instances fetch their config from a hub over TLS, with local ZeFS backup for partition resilience.

---

## Vision

A managed client's config contains everything: BGP sessions, plugin settings, AND the hub connection info. The `plugin { hub { client <name> { host; port; secret } } }` block declares who the client is and where its hub lives. After the first boot (provisioned via `ze init`), the cached config is self-describing -- `ze daemon` reads it and knows everything.

Every ze instance has at least one `server` block for local plugins and SSH. The hub infrastructure is universal -- managed config is just one more use of it.

---

## Hub Block Design

### Unified Named Blocks

All hub configuration uses explicit `server` and `client` keywords under `plugin { hub { } }`:

| Keyword | Direction | Has `host`+`port` | Purpose |
|---------|-----------|-------------------|---------|
| `server <name>` | Accepts connections | Listen address | Local plugins, SSH editor, remote managed clients |
| `client <name>` (under `server`) | N/A | N/A | Declares an accepted remote client with its secret |
| `client <name>` (at hub level) | Connects outbound | Remote address | Fetch config from a remote hub |

### Examples

**Standalone instance** (local plugins only):

```
plugin {
    hub {
        server local {
            host 127.0.0.1;
            port 1790;
            secret "local-secret-...";
        }
    }
}
```

**Central hub** (serves local plugins AND remote managed clients):

```
plugin {
    hub {
        server local {
            host 127.0.0.1;
            port 1790;
            secret "local-plugin-secret-...";
        }
        server central {
            host 0.0.0.0;
            port 1791;
            secret "remote-plugin-secret-...";
            client edge-01 { secret "..."; }
            client edge-02 { secret "..."; }
        }
    }
}
```

**Managed client** (local plugins + remote config):

```
plugin {
    hub {
        server local {
            host 127.0.0.1;
            port 1790;
            secret "local-secret-...";
        }
        client edge-01 {
            host 10.0.0.1;
            port 1791;
            secret "...";
        }
    }
}
```

### Key Rules

- Every ze instance has at least one `server` block (for local plugins and SSH)
- A `server` block listens; a hub-level `client` block connects outbound
- `client` blocks nested under `server` declare accepted remote clients (with per-client secrets)
- Hub-level `client` blocks declare outbound connections to remote hubs
- A block has either `host`+`port` for listening or `host`+`port` for connecting -- the keyword (`server`/`client`) determines which
- Multiple `server` blocks allowed (different secrets for different plugin groups)
- The client name in `client edge-01 { }` IS the client's identity

---

## Architecture

### No New Server

The hub (`plugin { hub { } }`) already provides:
- TLS listener with certificate management
- Token authentication (`#0 auth {"token":"...","name":"..."}`)
- MuxConn multiplexed RPCs (`#id verb [json]\n`)
- Connection tracking by name

Managed configuration adds:
- Per-client secrets in `server` blocks (instead of one shared secret)
- `config-fetch` and `config-changed` RPCs handled by the hub
- Client configs stored as entries in the hub's ZeFS blob

### Roles

| Role | Config | Description |
|------|--------|-------------|
| Hub (serves config) | `server central { client edge-01 { secret } }` | Accepts managed clients, serves their configs |
| Managed client (provisioning) | `ze init` with managed=true, hub host/port, token | One-time setup, stored in blob |
| Managed client (running) | `client edge-01 { host; port; secret }` in cached config | Connects to hub, fetches config |
| Standalone | Only `server local { }` blocks, no hub-level `client` | Local plugins, no remote hub |

`ze init` provisions the blob with identity and hub connection info. `ze daemon` reads the blob: if managed, connect to hub and fetch config. After the first fetch, the cached config is self-describing (contains the `client` block).

CLI flags (`--server`, `--name`, `--token`) can override blob/config values for troubleshooting, but `ze init` is the primary provisioning path.

### Per-Client Secrets

At `#0 auth`, the hub looks up the token against the `client` entries nested under the relevant `server` block. A client cannot use another client's token.

| Server config | Client config | Effect |
|--------------|--------------|--------|
| `server central { client edge-01 { secret "abc"; } }` | `client edge-01 { secret "abc"; }` | Match, accepted |
| `server central { client edge-01 { secret "abc"; } }` | `client edge-01 { secret "xyz"; }` | Mismatch, rejected |
| No `client edge-01` in any server | Any | Rejected: unknown client |

### Config Storage (Hub Side)

Client configs are entries in the hub's ZeFS blob, keyed by client name. The exact key format follows the blob namespace convention (see `spec-blob-namespaces`).

The admin manages these using existing blob tools (`ze db ls`, `ze config edit`, SSH editor).

### Config Storage (Client Side)

The managed client's local ZeFS blob caches the config received from the hub. The config itself contains everything needed:

| What | Where |
|------|-------|
| Client name | Block name in `client <name> { }` at hub level |
| Hub address | `host` + `port` in the same block |
| Auth token | `secret` in the same block |
| BGP config | `bgp { }` block |
| Local plugins | `server local { }` block |
| Managed mode toggle | `meta/instance/managed` in blob (see `spec-blob-namespaces`) |

The `meta/instance/managed` flag controls whether the client actually connects to the hub. Toggling it severs or establishes the hub connection without changing the config.

---

## Protocol

### RPCs

After `#0 auth`, the hub knows the client's name. All RPCs are scoped to that name implicitly.

| Verb | Direction | Payload | Response |
|------|-----------|---------|----------|
| `config-fetch` | Client to hub | `{"version":"<hash-or-empty>"}` | `ok {"version":"<hash>","config":"<base64>"}` or `ok {"status":"current"}` |
| `config-changed` | Hub to client | `{"version":"<hash>"}` | `ok {}` |
| `config-ack` | Client to hub | `{"version":"<hash>","ok":true}` or `{"version":"<hash>","ok":false,"error":"..."}` | `ok {}` |
| `ping` | Either direction | `{}` | `ok {}` |

### Version Hash

Truncated SHA-256 of the config bytes (hex-encoded, first 16 characters). Computed from content, never stored separately. Same content = same hash.

### Connection Lifecycle

1. Client reads cached config from local blob (or uses blob metadata from `ze init` on first boot)
2. Client extracts name, host, port, secret from hub-level `client <name> { }` block
3. Client connects via TLS to hub address
4. Client sends `#0 auth` with token and name
5. Hub validates token against `client` entry nested under the relevant `server` block
6. Client sends `config-fetch` with current version hash (or empty on first boot)
7. Hub reads the client's config from its blob, computes hash
8. If hashes match: `{"status":"current"}`; if different: full config in response
9. Client writes config to local blob, starts or reloads BGP
10. Heartbeat: `ping` every 30 seconds, timeout after 3 missed (90 seconds)
11. On config edit: hub sends `config-changed` to the connected client

### Config Change (Two-Phase)

| Phase | Action |
|-------|--------|
| 1. Notify | Hub sends `config-changed {"version":"<new-hash>"}` to connected client |
| 2. Fetch | Client sends `config-fetch {"version":"<new-hash>"}` when ready |
| 3. Validate | Client parses and validates the new config |
| 4. Apply | If valid: write to blob, reload BGP, send `config-ack {"ok":true}` |
| 5. Reject | If invalid: send `config-ack {"ok":false,"error":"..."}`, keep running |

The client controls timing. A router in the middle of graceful restart or convergence is not forced to reload.

---

## Client Startup Sequence

### First Boot (After `ze init`)

`ze init` has stored identity, managed flag, hub host/port, and token in the blob. No config exists yet.

| Step | Action |
|------|--------|
| 1 | `ze daemon` |
| 2 | Read blob: `meta/instance/name`, `meta/instance/managed`=true, hub host/port, token |
| 3 | Connect to hub, authenticate |
| 4 | Fetch config (includes `server local { }` + `client edge-01 { }` + `bgp { }`) |
| 5 | Write config to local blob |
| 6 | Start local hub server for plugins, start BGP |

After this, the blob has the cached config. On subsequent boots, the config itself provides hub connection info.

### Subsequent Boots

| Step | Action |
|------|--------|
| 1 | `ze daemon` (no flags) |
| 2 | Read cached config from local blob |
| 3 | Start local hub server (from `server local { }` block) |
| 4 | Extract name, host, port, secret from hub-level `client <name> { }` block |
| 5a | **Connection succeeds:** fetch latest config, update blob if newer, reload BGP if changed |
| 5b | **Connection fails:** start BGP from cached config, reconnect in background |

### Startup Matrix

| Hub | Cached config | Behavior |
|-----|--------------|----------|
| Reachable | Any | Fetch from hub, update blob, start BGP |
| Unreachable | Exists | Start from cached config, reconnect in background |
| Unreachable | Missing | First boot after init: exit with error (hub required) |

### Reconnect

| Parameter | Value |
|-----------|-------|
| Initial delay | 1 second |
| Backoff multiplier | 2x |
| Maximum delay | 60 seconds |
| Jitter | 10% random |

---

## Configuration

### YANG Structure

| Path | Type | Description |
|------|------|-------------|
| `plugin/hub` | container | Hub configuration |
| `plugin/hub/server` | list, keyed by name | Hub server instances |
| `plugin/hub/server/<name>/host` | string | Listen address |
| `plugin/hub/server/<name>/port` | uint16 | Listen port |
| `plugin/hub/server/<name>/secret` | string (min 32 chars) | Shared secret for plugins connecting to this server |
| `plugin/hub/server/<name>/client` | list, keyed by name | Accepted remote managed clients |
| `plugin/hub/server/<name>/client/<name>/secret` | string (min 32 chars) | Per-client auth token |
| `plugin/hub/client` | list, keyed by name | Outbound hub connections |
| `plugin/hub/client/<name>/host` | string | Remote hub address |
| `plugin/hub/client/<name>/port` | uint16 | Remote hub port |
| `plugin/hub/client/<name>/secret` | string (min 32 chars) | Auth token |

### Client Side

Two sources of hub connection info, used at different stages:

| Stage | Source | Contains |
|-------|--------|----------|
| First boot (after `ze init`) | Blob metadata | `meta/instance/name`, hub host/port, token |
| Subsequent boots | Cached config | `client <name> { host; port; secret }` block |

The cached config takes over once fetched. The blob metadata from `ze init` is the bootstrap that gets the first config.

CLI flag overrides (optional, for troubleshooting):

| Flag | Env var | Description |
|------|---------|-------------|
| `--server <host:port>` | `ze.managed.server` | Override hub address |
| `--name <name>` | `ze.managed.name` | Override client name |
| `--token <token>` | `ze.managed.token` | Override auth token |
| `--connect-timeout <dur>` | `ze.managed.connect.timeout` | Connection timeout (default: 5s) |

Priority: CLI flag > env var > config block > blob metadata.

---

## Security

| Concern | Mitigation |
|---------|-----------|
| Token per client | Each client has its own secret; token bound to name at auth |
| Token in config | Config blob permissions 0600; token not logged |
| TLS MITM | TLS 1.3 minimum; optional cert pinning (see `spec-plugin-tls-hardening.md`) |
| Client impersonation | Per-client secret + name binding; one connection per name |
| Config isolation | Client can only fetch its own config (name implicit from auth session) |
| Multiple servers | Different secrets for different trust levels (local vs. remote) |

---

## What's New vs. Reused

| Component | Status |
|-----------|--------|
| Hub TLS listener | Reuse as-is (now per `server` block) |
| Hub auth (`#0 auth`) | Extend: per-client secret lookup under `server` block |
| Hub MuxConn | Reuse as-is |
| Hub ZeFS blob | Reuse as-is (client configs are entries) |
| `ze db rm` | Reuse as-is |
| Named `server`/`client` hub blocks | **New:** replaces flat `listen`/`secret` fields |
| `config-fetch` / `config-changed` RPCs | **New:** hub-side handlers |
| Config change watcher | **New:** hub notifies on blob write |
| Managed client component | **New:** connection, fetch, cache, reconnect |
| `ze daemon` managed mode | **New:** detect managed from blob metadata or cached config |

---

## Non-Goals

| Not included | Why |
|-------------|-----|
| New server component | Hub already exists; extend it |
| Special metadata blob keys for hub info | Config block declares hub connection; `ze init` blob keys are bootstrap only |
| Multi-hub replication | One hub sufficient; HA via client cached config |
| Incremental config updates | Full config on change; configs are small (< 10KB typically) |

---

## Implementation

Reference spec: `plan/spec-fleet-config.md`

| Package | Purpose |
|---------|---------|
| `pkg/fleet/` | Shared types: version hash, config envelope, RPC verbs |
| `internal/component/plugin/server/` | Hub extensions: named server/client blocks, per-client auth, config-fetch handler |
| `internal/component/managed/` | Client: connection manager, reconnect, heartbeat |
| `cmd/ze/` | `ze daemon` managed mode detection + CLI flag overrides |
