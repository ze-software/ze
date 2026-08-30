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
    session { asn { local 65000; } }

    group upstream {
        session {
            family {
                ipv4/unicast { prefix { maximum 1000000; } }
                ipv6/unicast { prefix { maximum 200000; } }
            }
        }

        peer transit-a {
            connection {
                local  { ip auto; }
                remote { ip 10.0.0.1; }
            }
            session { asn { remote 65001; } }
        }
    }
}

plugin {
    internal rib { use bgp-rib; }
}
```

A peer holds two containers. `connection` holds the transport settings: the
local and remote address, the port, MD5, and TTL security. `session` holds the
protocol settings: the AS numbers, the router ID, the address families, and the
capabilities. Both are inherited from the group.

An internal plugin is a named entry whose `use` leaf gives the built-in name, so
one plugin can run more than once under different names.
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
| `connection { remote { ip; } }` | Peer IP address, or `dynamic` on a group | Yes |
| `connection { local { ip; } }` | Local bind address (`auto` selects it) | Yes |
| `session { asn { local; remote; } }` | Local and remote AS number | Yes (or inherited) |
| `session { router-id }` | BGP router ID for this peer | No (inherits `bgp/router-id`) |
| `description` | Human-readable description | No |
| `timer { receive-hold-time; send-hold-time; keepalive; connect-retry; }` | Timer container | No |
| `connection { remote { connect } }` | Initiate outbound TCP (`true`/`false`, default: `true`) | No |
| `connection { local { accept } }` | Accept inbound TCP (`true`/`false`, default: `true`) | No |
| `connection { remote { port } }`, `connection { local { port } }` | TCP port (default: 179) | No |
| `connection { md5 { password; ip; } }` | TCP MD5 authentication (RFC 2385) | No |
| `connection { ttl { max; set; min; } }` | TTL security / GTSM (RFC 5082) | No |
| `capture { }` | Protocol event capture, off by default | No |
| `attach { process <name> { } }` | Programs this peer feeds and accepts messages from | No |
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

Configured under `session { family { } }`. Each family requires a
`prefix { maximum N; }` block.

```
session {
    family {
        ipv4/unicast { prefix { maximum 1000000; } }
        ipv6/unicast { prefix { maximum 200000; } }
        l2vpn/evpn   { prefix { maximum 10000; } }
    }
}
```

IPv4/IPv6 unicast and multicast are built into the engine. Other families
(VPN, FlowSpec, EVPN, VPLS, BGP-LS, MPLS, MUP, MVPN, RTC) are loaded
automatically when you configure the corresponding family.
<!-- source: internal/component/bgp/plugins/nlri/ -- NLRI plugin registrations with Families field -->

### A Peer That Declares No Family

The block can be omitted. Such a peer advertises no Multiprotocol capability,
and RFC 4271 carries IPv4 unicast in the UPDATE message itself, so the session
exchanges IPv4 unicast. Ze negotiates that one family for it and sends the
End-of-RIB marker RFC 4724 Section 4 requires for it. The same rule applies to
the peer: a peer that advertises no Multiprotocol capability is read as
supporting IPv4 unicast.

A plugin fills the same gap. When the config declares no family, Ze advertises
the families the loaded plugins can decode. Config families win: a peer with a
`family` block ignores the plugin families.
<!-- source: internal/core/bgp/capability/negotiated.go -- Negotiate treats a side with no Multiprotocol capability as advertising ipv4/unicast -->
<!-- source: internal/component/bgp/reactor/session_negotiate.go -- buildOpen falls back to the plugin decode families -->

### Prefix Limits

The `prefix { }` block of one family holds the limit and what happens when the
peer crosses it.

| Leaf | Type | Default | Description |
|------|------|---------|-------------|
| `maximum` | uint32, 1 to max | (none) | Hard maximum number of prefixes accepted |
| `warning` | uint32 | 90% of `maximum` | Warning threshold |
| `teardown` | boolean | `true` | End the session when `maximum` is crossed. `false` warns only |
| `count` | `offered` or `installed` | `offered` | Which prefixes the count holds |
| `idle-timeout` | uint16 seconds | `0` | Wait before reconnect after this family ended the session |
| `reconnect` | `never`, `backoff` or `timer` | (none) | What the peer does after this family ended the session |

`count offered` counts wire events: every announcement raises the count and
every withdrawal lowers it. A peer that re-announces one prefix to change an
attribute raises the count again, so the count can sit above the maximum.
`count installed` counts the set of prefixes the family currently holds, which
no re-announcement and no unmatched withdrawal can move.

`idle-timeout` doubles on each repeat teardown, up to one hour. An unset
`reconnect` means `timer` when `idle-timeout` is more than 0, and `never` when
it is 0.

```
session {
    family {
        ipv4/unicast {
            prefix {
                maximum      1000000;
                count        installed;
                idle-timeout 60;
                reconnect    timer;
            }
        }
    }
}
```
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- container prefix -->

## Dynamic Peer Groups

A group whose `connection/remote/ip` is `dynamic` opens its own listening
socket and builds one peer for each accepted connection. The member inherits
the whole resolved group, its families, its filters and its `attach process`
blocks included.

```
group ix {
    connection {
        local  { ip auto; }
        remote {
            ip        dynamic;
            connect   false;
            range     [ 198.51.100.0/24 2001:db8:1::/48 ];
            max-peers 500;
        }
    }
    session {
        asn { remote 65002; }
        family { ipv4/unicast { prefix { maximum 500000; } } }
    }
}
```

| Leaf | Type | Default | Description |
|------|------|---------|-------------|
| `ip dynamic` | union with `enum dynamic` | (none) | Group level only. A peer that states it is skipped with a warning |
| `connect` | boolean | `true` | Must be stated as `false`. A dynamic group with no `connect` leaf is rejected |
| `range` | leaf-list of IP prefix | (none) | Accept a session from any address inside these prefixes. At least one is needed |
| `max-peers` | uint32, 1 to 100000 | `1000` | Maximum number of dynamic members of this group |

Overlapping ranges are resolved by longest prefix match. Each member is named
`dyn-<address>`.
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- container connection/remote -->

## Blackhole (RFC 7999)

The `blackhole { }` container states the RFC 7999 agreement on one BGP session,
in both directions. It is available at the `bgp`, `group` and `peer` levels.
Honoring is off by default.

```
peer transit-a {
    blackhole {
        communities [ blackhole 65001:666 ];
        prefixes    [ 192.0.2.0/24 ];
    }
}
```

| Leaf | Type | Default | Description |
|------|------|---------|-------------|
| `communities` | leaf-list of string, cumulative | (none) | Communities that ask Ze to discard traffic sent toward a prefix this peer announces. `blackhole` and `65535:666` are one value |
| `prefixes` | leaf-list of string, cumulative | (none) | Prefixes this peer is authorized to blackhole within |

RFC 7999 Section 3.3 states two conditions and both must hold. A received route
is honored only when it carries a listed community and a listed prefix covers it
with an equal or shorter length. A container with neither list discards nothing.
A `prefixes` list with no `communities` list resolves the community to the
well-known `65535:666`, because naming those prefixes is the explicit
configuration directive Section 4 asks for. A stated `communities` list is taken
exactly, and `65535:666` is not added to it.

The same list gates the send side. `announce blackhole` and
`announce unicast <prefix> community 65535:666` reach only the sessions that
named the well-known value, and they name the refused peers when no selected
peer agreed.

An honored route becomes a discard route in the Linux FIB, or a drop in VPP. It
needs no next-hop and no pre-created static route.
<!-- source: internal/component/bgp/plugins/rib/yang/ze-rib.yang -- grouping blackhole-honor-fields -->

RPKI origin validation can reject a blackhole announcement, because a blackhole
prefix is as long as possible while the covering ROA carries its maxLength at the
aggregate. The `rpki { blackhole-exempt }` leaf answers that. It is available at
the `peer` and `group` levels only, and there is no global level for it.

```
peer transit-a {
    rpki { blackhole-exempt true; }
}
```

| Leaf | Type | Default | Description |
|------|------|---------|-------------|
| `blackhole-exempt` | boolean | `false` | Keep a route carrying the BLACKHOLE community when its only validation fault is that the prefix is longer than a covering VRP allows |

The exemption is narrow. It applies only when a covering VRP names the route's
own origin AS and disagrees on nothing but length. A wrong origin AS stays
Invalid.
<!-- source: internal/component/bgp/plugins/rpki/yang/ze-rpki.yang -- grouping rpki-peer-policy -->

## Community Filters

The ingress community filter is available at the `bgp`, `group` and `peer`
levels, inside the existing `filter { }` container. It applies to external
sessions.

```
peer transit-a {
    filter {
        ingress {
            community {
                relation-tag          true;
                relation-function     3;
                scrub-own-ga          true;
                scrub-keep-function   [ 100 200 ];
                blackhole-propagation no-export;
            }
        }
    }
}
```

| Leaf | Type | Default | Description |
|------|------|---------|-------------|
| `relation-tag` | boolean | `false` | Tag each received route with the relation this session has to the local AS (RFC 8195) |
| `relation-function` | uint32 | `3` | Function number the relation tag writes under the local AS |
| `scrub-own-ga` | boolean | `false` | Remove received communities whose Global Administrator is the local AS (RFC 7454 Section 11) |
| `scrub-keep-function` | leaf-list of uint32, cumulative | (none) | Function numbers a peer may still send under the local AS. An empty list keeps nothing |
| `blackhole-propagation` | `none`, `no-export` or `no-advertise` | `none` | Community added to a received route that carries BLACKHOLE, to bound how far it travels |

The `relation-function` number is never kept by `scrub-keep-function`, whatever
that list says. A kept relation function would let a peer state its own relation
to the local AS.

The existing `ingress { community { tag; strip; } }` and
`egress { community { tag; strip; } }` leaf-lists are unchanged.
<!-- source: internal/component/bgp/plugins/filter_community/yang/ze-filter-community.yang -- grouping community-filter-fields -->

## Route Modification Policy

A `modify` policy carries its own `match` container, so one policy can change
only the routes that carry a stated community. A route that meets no stated
value passes through unchanged rather than being rejected. An absent `match`
container applies the operations to every route.

```
bgp {
    policy {
        modify scrub-med {
            match {
                community          [ 65001:100 ];
                large-community    [ 65001:100:200 ];
                extended-community [ target:65000:100 ];
            }
            del { med; }
        }
    }
}
```

| Leaf | Type | Default | Description |
|------|------|---------|-------------|
| `match/community` | leaf-list of string | (none) | Match the COMMUNITIES attribute (RFC 1997) |
| `match/large-community` | leaf-list of string | (none) | Match the Large Communities attribute (RFC 8092) |
| `match/extended-community` | leaf-list of string | (none) | Match the Extended Communities attribute (RFC 4360) |
| `del/med` | empty | (none) | Remove the MULTI_EXIT_DISC attribute (RFC 4271 type 4) from the route |

`del { med; }` is the mechanism RFC 4271 Section 5.1.4 requires. It works on
an import chain only. An export chain refuses it and logs why, because RFC 4271
Section 9.1.2.2 makes MULTI_EXIT_DISC part of the decision process.

The `set { }` container holds `local-preference`, `med`, `origin`, `next-hop`,
`as-path-prepend`, and the community add and remove leaf-lists. The `del { }`
container holds `med`. The `increment { }` and `decrement { }` containers are unchanged.
<!-- source: internal/component/bgp/plugins/filter_modify/yang/ze-filter-modify.yang -- list modify -->

## Protocol Event Capture

A peer can write every inbound message to a bounded JSONL file as raw wire
bytes, together with the config operations applied while the capture runs.
`ze-test replay <file>` drives the same read path over the same input with an
injected clock. Capture is a diagnostic aid, so leave it off in steady state.

```
peer transit-a {
    capture {
        enabled      true;
        directory    /var/lib/ze/capture;
        maximum-size 100;
        on-limit     rotate;
    }
}
```

| Leaf | Type | Default | Description |
|------|------|---------|-------------|
| `enabled` | boolean | `false` | Capture this peer's inbound protocol events |
| `directory` | string, 1 to 4096 characters | `/var/lib/ze/capture` | Directory holding the files, created if absent. One file per peer, named `bgp-<peer-address>.jsonl` |
| `maximum-size` | uint32 megabytes, 1 to 1024 | `100` | Hard cap on one capture file |
| `on-limit` | `rotate` or `stop` | `rotate` | `rotate` renames the file to `<file>.1` and starts a fresh one, so the newest events are kept. `stop` closes the file, so the oldest events are kept |

The file holds the peer's routing data. It holds no local secret: TCP MD5 keys
never reach the wire, and captured config payloads are redacted. Handle a
capture file like a routing-table dump. `ze doctor` reports whether the
directory is writable (`doctor-bgp-capture-directory`).
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- container capture -->

## Plugin Loading

Plugins are declared in the `plugin { }` block:

```
plugin {
    internal rib { use bgp-rib; }
    internal gr  { use bgp-gr; }
    external my-plugin {
        run "/path/to/binary";
        encoder json;
    }
}
```

`internal` plugins run as goroutines via DirectBridge (zero IPC overhead).
`external` plugins run as separate processes with TLS connect-back. Each entry
is keyed by the name you give it, and the `use` leaf names the built-in plugin
that runs under it. An `internal` entry with no `use` leaf is rejected.

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
        receive [ update state refresh ];
        send    [ update ];
    }
}
```

This block replaces the older `process <name> { }` form written directly on the
peer. The `attach` container is flattened, so the statement is written as one
line: `attach process <name> { ... }`.

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
| `congested` | The peer's send queue filled |
| `resumed` | The peer's send queue drained again |
| `rpki` | RPKI validation results |
| `listener-ready` | The TCP listener is bound and accepting |
| `update-notification` | Lightweight arrival notice for an UPDATE |
| `*` | Every registered type, in both directions |

A plain type is fed in both directions. Add `-received` or `-sent` to name one:
`receive [ update-received ]` asks for the UPDATEs the peer sends to ze.

| Send type | Permits |
|-----------|---------|
| `update` | originating routes toward the peer |
| `refresh` | asking the peer to re-advertise |
| `raw` | writing a whole BGP message the program built itself |
| `*` | all three, and every send type registered later |

Those three are the base types. A plugin registers more, and naming one in a send
list auto-loads the plugin that enables it: `send [ enhanced-refresh ]` starts
bgp-route-refresh.

Those two are read at delivery, in both directions. A peer that attaches no
program feeds it no peer-scoped event, and a program that the peer does not
list in `send` cannot originate a message toward it.

An optional `content { }` container states how the events are rendered for one
program:

```
peer transit-a {
    attach process watcher {
        receive [ update-received state ];
        send    [ update ];
        content { encoding json; format raw; attribute minimal; }
    }
}
```

| Leaf | Type | Description |
|------|------|-------------|
| `encoding` | string | Wire encoding for the events, for example `json` or `text` |
| `format` | string | Output format template for event rendering |
| `attribute` | string | Which BGP attributes the events carry |

A block written on a group reaches every peer of that group, dynamic members
included. A member that restates the block replaces the group's list for
itself. `show event delivery` prints the resulting edges.
<!-- source: internal/core/bgp/events/events.go -- event and send type names -->
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- container attach -->
<!-- source: internal/component/bgp/reactor/peer_settings.go -- ProcessBinding -->

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
    ca corp-root {
        certificate "<base64-DER>";
    }
    certificate lan {
        certificate  "<base64-DER>";
        intermediate [ "<base64-DER>" ];
        private { key "<base64-DER>"; }
    }
}
```

X.509 certificates and private keys for IPsec and TLS. Each value is a
base64-encoded DER certificate, with no PEM header lines. The private key is
stored as `$9$` in the config file and is decoded on load. PKCS8, SEC1 (ECDSA)
and PKCS1 (RSA) key encodings are read.

| Node | Type | Description |
|------|------|-------------|
| `ca <name> { certificate }` | string, mandatory | Trusted CA certificate |
| `certificate <name> { certificate }` | string, mandatory | Device certificate |
| `certificate <name> { intermediate }` | leaf-list of string, ordered | Intermediate CAs, in order from the issuer of this certificate toward the trust anchor |
| `certificate <name> { private { key } }` | string, sensitive | Private key for the device certificate |

RFC 7296 Section 3.6 asks an implementation to be able to send up to four X.509
certificates. This device certificate plus three intermediates is that maximum.

Certificates are validated at commit time. Health monitoring reports expiry
warnings (30 days) and errors (expired).
<!-- source: internal/component/pki/yang/ze-pki-conf.yang -- container pki -->

### TLS Listeners

The web server, and the DoT and DoH listeners of `as112` and `geodns`, serve a
certificate named in the PKI store. Each listener sends the leaf certificate and
every intermediate the store entry holds, so a client can build the full chain.

```
environment {
    web {
        enabled     true;
        certificate lan;
        server main { ip 0.0.0.0; port 3443; }
    }
}

service {
    as112  { tls { certificate lan; } }
    geodns { tls { certificate lan; } }
}
```

| Leaf | Type | Default | Description |
|------|------|---------|-------------|
| `environment/web/certificate` | string, 1 to 255 characters, `[A-Za-z0-9._-]` | (none) | PKI store entry served on the HTTPS listener |
| `service/as112/tls/certificate` | string, same pattern | (none) | PKI store entry served on DoT and DoH |
| `service/geodns/tls/certificate` | string, same pattern | (none) | PKI store entry served on DoT and DoH |

An unset `environment/web/certificate` means Ze generates a self-signed
certificate and serves that. A name the store does not hold, or an entry with no
private key, stops the web server and rejects the reload. Ze never falls back to
a self-signed certificate for a configured name. A reload rotates the material
through a per-handshake lookup, with no rebind. The environment variable
`ze.web.certificate` overrides the config file.

For `as112` and `geodns`, `certificate` and the older `cert-file` / `key-file`
pair are mutually exclusive, and setting both is a configuration error. The PKI
store lives in the hub process, so an externally started `as112` or `geodns`
cannot resolve a store name. Its secure listeners then do not start and the
error is logged. Cleartext DNS is unaffected.
<!-- source: internal/component/web/yang/ze-web-conf.yang -- leaf certificate -->
<!-- source: internal/plugins/as112/yang/ze-as112-conf.yang -- container tls -->
<!-- source: internal/plugins/geodns/yang/ze-geodns-conf.yang -- container tls -->

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

### IPv6 Router Advertisements

Ze sends Router Advertisements (RFC 4861) on an interface unit, which is the
role radvd fills on other systems. The container is Linux and netlink only. A
`backend vpp` tree that carries it is rejected at config verify.

```
interface {
    backend netlink;
    ethernet eth0 {
        unit 0 {
            ipv6 {
                address [ 2001:db8:1::1/64 ];
                forwarding true;
                router-advertisement {
                    enabled true;
                    maximum-interval 900;
                    minimum-interval 300;
                    router-lifetime 1800;
                    hop-limit 64;
                    managed false;
                    other-config false;
                    prefix 2001:db8:1::/64 {
                        on-link true;
                        autonomous true;
                        valid-lifetime 86400;
                        preferred-lifetime 43200;
                    }
                    rdnss {
                        server [ 2001:db8:1::53 2001:db8:1::54 ];
                        lifetime 3600;
                    }
                }
            }
        }
    }
}
```

| Leaf | Type | Default | Description |
|------|------|---------|-------------|
| `enabled` | boolean | `false` | Send advertisements on this unit |
| `maximum-interval` | uint16 seconds, 4 to 1800 | `600` | Longest wait between unsolicited advertisements |
| `minimum-interval` | uint16 seconds, 3 to 1350 | `200` | Shortest wait between unsolicited advertisements |
| `router-lifetime` | uint16 seconds, 0 to 9000 | `1800` | How long hosts keep Ze in their default router list. 0 means Ze is no default router, and the prefixes and resolvers still apply |
| `hop-limit` | uint8 | `64` | Hop limit hosts use on their own packets |
| `managed` | boolean | `false` | The M flag: hosts get addresses from DHCPv6 |
| `other-config` | boolean | `false` | The O flag: hosts get other settings from DHCPv6 |
| `reachable-time` | uint32 milliseconds, 0 to 3600000 | `0` | AdvReachableTime. 0 states no value |
| `retransmit-timer` | uint32 milliseconds | `0` | AdvRetransTimer. 0 states no value |

Each `prefix` list entry is keyed by the prefix:

| Leaf | Type | Default | Description |
|------|------|---------|-------------|
| `on-link` | boolean | `true` | The L flag: hosts treat addresses in this prefix as on-link |
| `autonomous` | boolean | `true` | The A flag: hosts build addresses from this prefix by SLAAC |
| `valid-lifetime` | uint32 seconds | `2592000` | How long the prefix stays valid. 30 days by default |
| `preferred-lifetime` | uint32 seconds | `604800` | How long addresses from it stay preferred. It must not exceed `valid-lifetime` |

SLAAC needs a 64-bit prefix (RFC 4862 Section 5.5.3). The `rdnss` container
advertises resolvers with the RFC 8106 option:

| Leaf | Type | Default | Description |
|------|------|---------|-------------|
| `server` | leaf-list of IPv6 address, at most 8 | (none) | Resolver addresses, all sharing one lifetime |
| `lifetime` | uint32 seconds | (none) | How long hosts may use these resolvers |

An unset `lifetime` sends 3 x `maximum-interval`, the value RFC 8106
Section 5.1 recommends. A `lifetime` of 0 tells hosts to stop using the
resolvers, which is why 0 is not the default: Ze could not otherwise tell an
operator retiring a resolver from one who set nothing.

Sending is separate from accepting. Set `forwarding true` on an advertising
interface, and leave `accept-ra` at 0 unless the same interface also learns
from another router. `ze doctor` reports `doctor-iface-ra-forwarding` for an
advertising interface whose `net.ipv6.conf.<dev>.forwarding` is 0.

There is no show command for this. Router Advertisements are observed through
Prometheus: `ze_iface_ra_sent_total{interface}` counts every advertisement, and
`ze_iface_ra_solicited_total{interface}` counts the subset that answers a Router
Solicitation.
<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- container router-advertisement -->
<!-- source: internal/plugins/iface/ra/sender_linux.go -- Router Advertisement send loop -->

### Interface Route Priority

`route-priority` is the metric of a default route the unit learns, from DHCP, a
Router Advertisement, or IPCP on a PPPoE client. Lower values are preferred.

| Leaf | Type | Default | Description |
|------|------|---------|-------------|
| `interface { <kind> <name> { unit <u> { route-priority } } }` | uint32, 0 to 4294966271 | `254` | Metric for default routes learned on this unit |
| `interface { pppoe-client <name> { route-priority } }` | uint32, 0 to 4294966271 | `254` | Metric for the default route installed after IPCP completes |

The default was 0 in earlier releases. 254 ranks a learned default below a
static or protocol-learned one. On link-down the metric is raised by 1024, which
deprioritizes the interface.
<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- leaf route-priority -->

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

### Subscriber Authentication

PPPoE and L2TP each carry the same authentication pair. The credential is
verified by the shared `l2tp-auth-local` or `l2tp-auth-radius` handler.

```
pppoe {
    auth-method   chap-md5;
    allow-no-auth false;
}

l2tp {
    auth-method   chap-md5;
    allow-no-auth false;
}
```

| Leaf | Type | Default | Description |
|------|------|---------|-------------|
| `auth-method` | `none`, `pap`, `chap-md5` or `ms-chap-v2` | `chap-md5` | PPP Auth-Protocol method first advertised in the LCP Configure-Request |
| `allow-no-auth` | boolean | `false` | Let LCP negotiation finish with no Auth-Protocol |

Set `auth-method none` only together with `allow-no-auth true`. With the default
`allow-no-auth false`, a peer that rejects every configured method is
disconnected rather than accepted with no authentication.
<!-- source: internal/component/l2tp/pppoe/yang/ze-pppoe-conf.yang -- leaf auth-method -->
<!-- source: internal/component/l2tp/yang/ze-l2tp-conf.yang -- leaf auth-method -->

### L2TP RADIUS NAS-Port-Id

An LNS has no physical port, so Ze composes the RFC 2869 Section 5.17
NAS-Port-Id from a template. It is resolved once per session, so the
Access-Request and every accounting record of one session carry the same text
and a billing system can join them.

```
l2tp {
    auth {
        radius {
            nas-identifier     "bng-1";
            nas-port-id-format "{nas-id}:{tunnel-id}.{session-id}";
        }
    }
}
```

| Leaf | Type | Default | Description |
|------|------|---------|-------------|
| `nas-port-id-format` | string, 1 to 253 characters | (none) | Template for NAS-Port-Id. Unset sends no attribute |

The placeholders are `{nas-id}`, `{tunnel-id}` and `{session-id}`, and every
other character is copied. The 253-octet limit is the largest value a RADIUS
attribute can carry (RFC 2865 Section 5). An unknown placeholder is refused at
commit time, and so is a `{nas-id}` with no `nas-identifier` set.
<!-- source: internal/component/l2tp/plugins/authradius/yang/ze-l2tp-auth-radius-conf.yang -- leaf nas-port-id-format -->

## Removed Syntax

| Form | State |
|------|-------|
| `env { }` at the top level | Parse error. `unknown top-level keyword: env` |
| `process <name> { }` written directly on a peer | Replaced by `attach process <name> { }` |

`ze config validate` always rejected a top-level `env { }` block. One daemon
path used to accept it, read it once, and discard it. Both now refuse it. Set
environment variables in the process environment instead.

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
