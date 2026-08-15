# Configuration Reference

A concise reference for Ze's configuration syntax. For the full guide with
examples for every feature, see [guide/configuration.md](guide/configuration.md).

## Syntax

Ze uses a JUNOS-like hierarchical format. YANG-driven parsing: each config
node's type (leaf, leaf-list, container, list) determines how it is parsed.
Unknown keys are rejected with a suggestion for the closest valid key.
<!-- source: internal/component/config/tokenizer.go -- tokenizer for JUNOS-like syntax -->
<!-- source: internal/component/config/yang/loader.go -- YANG-driven config parsing -->

| Element | Syntax | Example |
|---------|--------|---------|
| Blocks | `name { ... }` | `bgp { ... }` |
| Values | `key value` or `key value;` | `router-id 1.2.3.4` |
| Comments | `#` to end of line | `# this is a comment` |
| Lists | `[ item1 item2 ]` | `receive [ update state ]` |
| Strings | Unquoted or `"double quoted"` | `run "/usr/bin/my-plugin"` |
| Terminators | Optional semicolons | Both `router-id 1.2.3.4` and `router-id 1.2.3.4;` work |
| Inline blocks | `name { key value; key value; }` | `remote { ip 10.0.0.1; as 65001; }` |

Indentation is not significant.
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- BGP config YANG schema -->

## Minimal Example

```
bgp {
    router-id 1.2.3.4;
    local { as 65000; }

    group upstream {
        family {
            ipv4/unicast { prefix { maximum 1000000; } }
            ipv6/unicast { prefix { maximum 200000; } }
        }

        peer transit-a {
            remote { ip 10.0.0.1; as 65001; }
        }
    }
}

plugin {
    internal bgp-rib;
}
```
<!-- source: internal/component/bgp/config/resolve.go -- ResolveBGPTree -->

## Inheritance

Three-level deep-merge for containers:

| Level | Scope | Example |
|-------|-------|---------|
| BGP | All peers | `bgp { router-id 1.2.3.4; }` |
| Group | Peers in group | `group upstream { timer { receive-hold-time 180; } }` |
| Peer | Single peer | `peer transit-a { timer { receive-hold-time 90; } }` |

Containers (like `capability`, `family`, `timer`) are deep-merged across levels.
Leaf values override at the more specific level.
<!-- source: internal/component/bgp/config/resolve.go -- ResolveBGPTree, inheritance merging -->

## Peer Settings

Peers are keyed by name: `peer <name> { }` where the name must start with a letter.

| Setting | Description | Required |
|---------|-------------|----------|
| `remote { ip; as; }` | Peer IP and AS number | Yes |
| `local { ip; as; }` | Local bind address and AS | Yes (ip can be `auto`) |
| `router-id` | BGP router ID | Yes (or inherited) |
| `description` | Human-readable description | No |
| `timer { receive-hold-time; send-hold-time; connect-retry; }` | Timer container | No |
| `remote { connect }` | Initiate outbound TCP (`true`/`false`, default: `true`) | No |
| `local { accept }` | Accept inbound TCP (`true`/`false`, default: `true`) | No |
| `port` | TCP port (default: 179) | No |
| `md5-password` | TCP MD5 authentication (RFC 2385) | No |
| `ttl-security` | Minimum TTL for incoming packets (RFC 5082) | No |
<!-- source: internal/component/bgp/config/peers.go -- PeersFromTree -->
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- peer settings -->

## Capabilities

Configured under `capability { }` at any inheritance level.

| Capability | Config | Notes |
|------------|--------|-------|
| 4-byte ASN | `asn4` | Enabled by default |
| Route Refresh | `route-refresh` | RFC 2918 |
| Extended Message | `extended-message` | RFC 8654, raises max message to 65535 bytes |
| Graceful Restart | `graceful-restart { restart-time 120; }` | RFC 4724 |
| ADD-PATH | `add-path { direction send/receive; }` | RFC 7911, with optional `limit N` for PATHS-LIMIT |
| Extended Next Hop | `nexthop { ipv4/unicast ipv6; }` | RFC 8950, per-family NH mapping |
| BGP Role | `role provider` | RFC 9234: provider, customer, rs, rs-client, peer |
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- capability definitions -->

## Address Families

Configured under `family { }`. Each family requires a `prefix { maximum N; }` block.

```
family {
    ipv4/unicast { prefix { maximum 1000000; } }
    ipv6/unicast { prefix { maximum 200000; } }
    l2vpn/evpn { prefix { maximum 10000; } }
}
```

IPv4/IPv6 unicast and multicast are built into the engine. Other families
(VPN, FlowSpec, EVPN, VPLS, BGP-LS, MPLS, MUP, MVPN, RTC) are loaded
automatically when you configure the corresponding family.
<!-- source: internal/component/bgp/plugins/nlri/ -- NLRI plugin registrations with Families field -->

## Plugin Loading

Plugins are declared in the `plugin { }` block:

```
plugin {
    internal bgp-rib;
    internal bgp-gr;
    external my-plugin {
        run "/path/to/binary";
        encoder json;
    }
}
```

`internal` plugins run as goroutines via DirectBridge (zero IPC overhead).
`external` plugins run as separate processes with TLS connect-back.

BGP itself is config-driven: if `bgp { }` is present, BGP loads. If not, ze
starts without BGP. The same mechanism works for `interface { }`, `firewall { }`,
and other top-level sections. Plugins can be added or removed at runtime via
config reload (SIGHUP).
<!-- source: internal/component/plugin/server/startup_autoload.go -- config-driven plugin loading -->

## Process Bindings

Plugins receive BGP events through the `attach process` blocks on each peer.
A peer that attaches no block for a plugin feeds it nothing and lets it send
nothing.

```
peer transit-a {
    attach process rib {
        receive [ update state refresh ]
        send [ update ]
    }
}
```

| Event | Description |
|-------|-------------|
| `update` | Route announcements and withdrawals |
| `state` | Peer state changes (up/down) |
| `open` | OPEN message |
| `notification` | NOTIFICATION message |
| `keepalive` | KEEPALIVE message |
| `refresh` | Route refresh request |
| `negotiated` | Capability negotiation results |
| `eor` | End-of-RIB marker |
| `rpki` | RPKI validation results |
| `*` | Every registered type, in both directions |

A plain type is fed in both directions. Add `-received` or `-sent` to name one:
`receive [ update-received ]` asks for the UPDATEs the peer sends to ze.

| Send type | Permits |
|-----------|---------|
| `update` | originating routes toward the peer |
| `refresh` | asking the peer to re-advertise |
| `*` | both, and every send type registered later |

Those two are the base types. A plugin registers more, and naming one in a send
list auto-loads the plugin that enables it: `send [ enhanced-refresh ]` starts
bgp-route-refresh.

A block written on a group reaches every peer of that group, dynamic members
included. A member that restates the block replaces the group's list for
itself. `show event delivery` prints the resulting edges.
<!-- source: internal/component/bgp/event.go -- event type definitions -->
<!-- source: internal/component/bgp/reactor/peersettings.go -- ProcessBinding -->

## Configuration Database

Ze stores configuration in ZeFS, a netcapstring-framed blob store. Configuration
is managed through an interactive editor with draft/commit workflow, not by
editing text files and sending SIGHUP (though SIGHUP reload is also supported).
<!-- source: pkg/zefs/store.go -- ZeFS blob store -->

| Command | Purpose |
|---------|---------|
| `ze config edit` | Interactive editor with YANG-aware tab completion |
| `ze config validate <file>` | Validate a config file offline |
| `ze config import <file>` | Import a config file into the blob store |
| `ze config migrate <file>` | Convert ExaBGP config to ze-native format |
| `ze signal reload` | Trigger config reload (same as SIGHUP) |
<!-- source: internal/component/config/cli/main.go -- config domain commands -->

## System Configuration

```
system {
    name-server 8.8.8.8;
    dns {
        timeout 5;
        cache-size 1000;
    }
    peeringdb {
        url "https://www.peeringdb.com";
        margin 10;
    }
    update-check {
        url "https://archive.example.com/ze/version.json";
        interval 3600;
    }
}
```
<!-- source: internal/component/config/system/yang/ze-system-conf.yang -- system config -->

### Firmware Update Check

The `update-check` block configures periodic version checking against a remote manifest.
Ze fetches the URL and compares the remote version against its own release version
(lexicographic comparison, matching ze's `YY.MM.DD` date-based versioning).

Each request includes an `X-Ze-Arch` header (e.g., `linux/arm64`) so the server can
reject requests from incompatible architectures.

The remote endpoint must serve a JSON object with a `version` field:

```json
{"version": "26.05.17"}
```

| Leaf | Default | Description |
|------|---------|-------------|
| `url` | (none, feature disabled) | HTTPS URL of the version manifest. HTTP allowed only for localhost. |
| `interval` | 86400 (daily) | Check interval in seconds (range: 60 to 604800). |
| `auto-apply` | false | Enable automated download, SHA-256 verification, and binary staging. |
| `spread` | 3600 | Maximum random delay in seconds before download (deterministic per device+version). |
| `maintenance-window/start` | (none) | Start of binary replacement window (HH:MM). |
| `maintenance-window/end` | (none) | End of binary replacement window (HH:MM). Midnight crossing is valid. |
| `restart/time` | (none) | Scheduled restart time (HH:MM). Use `restart { immediate; }` for immediate. |

When a newer version is detected, a warning appears in `show warnings` and `show system update`.
With `auto-apply false` (default), Ze only reports availability. With `auto-apply true`, Ze downloads
the binary, verifies its SHA-256, and atomically replaces itself via rename with a `.prev` rollback backup.
See [guide/self-update.md](guide/self-update.md) for the full guide.

### Serving Updates

Ze includes a built-in command to serve the version manifest and binary from build/release infrastructure:

```
ze update serve --listen :8080
```

This starts a minimal HTTP server with three endpoints:

| Endpoint | Description |
|----------|-------------|
| `GET /` | Index page showing version, architecture, and available endpoints |
| `GET /version.json` | Version manifest (`{"version":"26.05.17"}`). Returns 404 if the request's `X-Ze-Arch` header does not match the server's architecture. |
| `GET /<goos>/<goarch>` | The running binary (e.g., `GET /linux/arm64`) |

The architecture check ensures a router only sees an update when a matching binary is
available. A mismatched request (e.g., `arm64` router checking an `amd64` server) gets
a 404, which the checker logs as "check failed" and retries at the next interval.

Run it on a build server after compiling Ze so that deployed routers can check for updates.
The router's web interface does not expose its own version (to avoid helping attackers fingerprint the device).

### PKI Certificate Store

```
pki {
    certificate <name> {
        certificate-file <path>;
        private-key-file <path>;
    }
    ca-certificate <name> {
        certificate-file <path>;
    }
}
```

X.509 certificates and private keys for IPsec and TLS. PEM format. Validated at commit time. Health monitoring reports expiry warnings (30 days) and errors (expired).
<!-- source: internal/component/pki/config.go -- PKI config parser -->

### IPsec VPN

```
vpn {
    ipsec {
        tunnel <name> {
            ike {
                version 2;
                proposal <name> { encryption aes256gcm16; dh-group modp2048; prf sha256; }
                remote-address <ip>;
                authentication { method certificate; certificate <pki-name>; ca-certificate <pki-name>; }
                dpd { interval 30; timeout 120; }
            }
            child <name> {
                esp-proposal { encryption aes256gcm16; }
                local-ts <cidr>;
                remote-ts <cidr>;
                start-action auto;
            }
        }
    }
}
```

Native IKEv2 engine with XFRM dataplane. See [guide/configuration.md](guide/configuration.md) for the full reference.
<!-- source: internal/component/ike/ipsec/config.go -- IPsec config parser -->

### XFRM Interfaces

```
interface {
    xfrm <name> {
        if-id <N>;
        unit main {
            ipv4 { address [ <cidr> ]; }
        }
    }
}
```

Route-based IPsec. The `if-id` must match the IPsec SA's interface ID.

### RPKI ASPA Policy

```
bgp {
    rpki {
        aspa {
            validation true;
            action {
                invalid reject;   # reject | log-only | accept
            }
        }
    }
}
```

ASPA path verification with configurable policy enforcement. Requires RTR v2 cache server.
The `action` block can also be set per-peer or per-group (peer > group > global, per leaf).

## Environment Variables

Ze uses typed, registered environment variables for runtime tuning. See
[guide/environment-variables.md](guide/environment-variables.md) for the full list.
<!-- source: internal/core/env/env.go -- env var registry -->

## Further Reading

| Topic | Document |
|-------|----------|
| Full configuration guide | [guide/configuration.md](guide/configuration.md) |
| Architecture overview | [architecture.md](architecture.md) |
| Plugin guide | [guide/plugins.md](guide/plugins.md) |
| Quick start | [guide/quickstart.md](guide/quickstart.md) |
| ExaBGP migration | [exabgp/exabgp-migration.md](exabgp/exabgp-migration.md) |
| Command reference | [guide/command-reference.md](guide/command-reference.md) |
