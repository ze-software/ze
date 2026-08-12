# AS112

Ze's `as112` plugin runs an AS112 anycast DNS node: an authoritative sink for misdirected RFC 1918 and link-local reverse-DNS queries (RFC 7534), plus the EMPTY.AS112.ARPA DNAME-redirection sink (RFC 7535). It answers on four fixed anycast host addresses, so the plugin config never needs an operator-supplied service IP.
<!-- source: internal/plugins/as112/register.go -- as112 registration -->

## Quick Start

```
service {
    as112 {
        enabled true
        hostname "node1.example.net"
    }
}
```

This is the full config for the DNS node. The four canonical addresses (192.175.48.1, 192.31.196.1, 2620:4f:8000::1, 2001:4:112::1) are Go constants, registered against the interface address-ownership registry and bound on `lo` automatically, so they are not typed into config.

`as112` must run as an **internal** plugin (the default auto-load path, or an explicit `plugin { internal as112 { use as112 } }`). It must not run as `plugin { external as112 { ... } }`. The address-ownership registration is a same-process call into the `iface` engine; as a forked subprocess, it would register against its own copy of that state and never reach the kernel. `as112` refuses to start and logs an error if it detects it is not running in-process.

## Configuration Reference

All leaves go under `service { as112 { ... } }`.

| Leaf | Type | Default | Description |
|------|------|---------|-------------|
| `enabled` | boolean | false | Enable the AS112 anycast DNS node. |
| `address-family` | enumeration | `both` | `both`, `ipv4-only`, or `ipv6-only`. Restricts the service to one address family (RFC 7534 §3.4 / RFC 7535 §3.1 single-stack option). |
| `hostname` | string (0-63) | (empty) | Node-identification string surfaced in HOSTNAME.AS112.NET/ARPA TXT answers, so operators can tell which anycast instance answered a query. Empty omits the TXT string. |
| `facility` | string (0-100) | (empty) | Facility/site name surfaced alongside `location` in the HOSTNAME TXT answer. |
| `location` | string (0-100) | (empty) | City/country surfaced alongside `facility`. |
| `allow-from` | leaf-list (ip-prefix) | (empty = answer all) | Optional client-source access list. When non-empty, only queries from a listed prefix are answered; the rest are silently dropped. Loopback/on-box sources are always permitted, so `request as112 healthcheck` is never blocked. Setting this makes the node non-public, which is correct for a local-use mirror and wrong for a globally-reachable AS112 contributor. |
| `asn` | asn (1-4294967295) | 112 | Origin AS the covering prefixes carry when redistributed into BGP (`import as112`). Defaults to the well-known AS112 number 112, since the source models an AS112 virtual router. Set an operator or RFC 6996 private ASN to originate under a coordinated or local-use AS. Ignored unless `import as112` is configured. |
| `community` | leaf-list (string) | (empty) | Optional BGP communities on the redistributed covering prefixes. Accepts AA:NN and well-known names (`no-export`, `nopeer`; RFC 1997/3765), the values RFC 7534 §3.4 recommends for restricting AS112 route propagation. Ignored unless `import as112` is configured. |
| `watchdog` | boolean | true | Health-gate the BGP announcement on DNS serving state (RFC 7534 §3.3). true (default) announces only while the node is serving and withdraws on serving loss, `enabled false`, or shutdown; false announces as soon as enabled and imported. Ignored unless `import as112` is configured. |
<!-- source: internal/plugins/as112/yang/ze-as112-conf.yang -- YANG leaves -->

The combined `hostname`+`facility`+`location` TXT payload is bounded so the assembled HOSTNAME.AS112.* response always fits 512 octets with TC=0, even at every field's maximum length (RFC 7534 §3.5).

`allow-from` is the recommended way to restrict a local-use mirror to known client ranges. Set it directly in `service { as112 { ... } }` rather than hand-authoring `firewall` section rules matching UDP/TCP port 53 across all four anycast addresses (two per family). Keeping access control in the plugin avoids splitting one policy decision across two subsystems, and the loopback/on-box carve-out is never blocked, which a hand-authored firewall rule would have to replicate correctly.

The on-box carve-out recognises both loopback and the node's own four anycast addresses as sources. The healthcheck probe deliberately queries the real anycast address rather than loopback; see [BGP Integration](#bgp-integration-conditional-origination). Unlike loopback, the anycast addresses are ordinary public IPs that a remote sender could forge as a UDP source, so a query claiming to originate from the node's own address bypasses `allow-from`. This does not enable data exfiltration because replies go to the real address rather than the spoofed one, but `allow-from` should not be the only access control against a source willing to spoof the node's own addresses.

## CLI Commands

| Command | Description |
|---------|-------------|
| `show as112` | Node status: enabled, address-family, hostname/facility/location, allow-from count, served zone count, current SOA serial, and the address-registry's health (`address-registry-ok`; when false, `address-registry-error`/`address-registry-error-at` explain the most recent failure to apply the anycast addresses to a kernel interface). |
| `request as112 healthcheck [target <ip>]` | One-shot authoritative query against an anycast service address (or the given target; defaults to the address-family-appropriate loopback). Exit 0 when the expected AS112 answer comes back. This is the tool a healthcheck probe or monitoring script calls, since `dig` is not on the gokrazy appliance and `ze resolve dns` cannot target a specific server. |
<!-- source: internal/plugins/as112/health.go -- handleAS112Health -->
<!-- source: internal/plugins/as112/show.go -- handleShowAS112 -->

## BGP Integration (Conditional Origination)

AS112 covering prefixes (the /24s and /48s per RFC 7534 §3.4 / RFC 7535 §3.1, NOT the /32,/128 host addresses bound on `lo`) can be originated into BGP two ways. Both preserve the layering rule: `as112` never reads `bgp {}` config and BGP hardcodes no AS112 knowledge.

| Path | When to use | Origin-AS / community control | Health gate |
|------|-------------|-------------------------------|-------------|
| **Redistribute** | Use when the same ASN, community, and service-state gate should apply to every BGP peer. One `import as112` line plus source-level `asn`/`community` in the `as112` block. | Source-level (`asn`, `community` leaves), identical on every peer. | Process serving-state (`watchdog` leaf, default true): announce while the DNS node serves. |
| **Hand-authored** | Use when per-peer origin-AS/community, dedicated peer-group policy, or end-to-end anycast-path liveness matters. | Per-peer, per-`update`-block (`as-path`, `community`, `replace-as`). | `healthcheck` probe gating a `watchdog` `update` block, which can probe the real anycast path as well as loopback. |

### Redistribute origination (same settings for every peer)

Model the AS112 node as a virtual router with its own ASN and let the four covering prefixes enter the BGP RIB like `static`/`connected` redistribution, triggered by one `import as112` line. The routes are announced only while the DNS node is serving (`watchdog` default true, RFC 7534 §3.3) and withdrawn on serving loss, `enabled false`, or shutdown.

```
service {
    as112 {
        enabled true
        asn 112              # origin AS on the wire (default 112); an operator/private ASN overrides
        community [ nopeer ] # optional; RFC 7534 §3.4 propagation restriction (RFC 1997/3765)
        # watchdog true      # default: announce only while serving
    }
}
redistribute {
    destination bgp {
        import as112         # the trigger; without this line nothing is announced
    }
}
bgp {
    # ordinary peer/group config. The covering prefixes are announced to all BGP
    # peers (including a peer that establishes later). Use egress community/prefix
    # filters, or the hand-authored path below, for per-peer scope.
}
```

To an iBGP peer the AS_PATH is `[asn]` (e.g. `[112]`); to an eBGP peer the local AS is prepended, giving `[localAS, asn]`. `ze doctor` warns (`doctor-as112-redistribute-origin-uncoordinated`) when `asn 112` reaches an eBGP session to a public ASN, an uncoordinated global AS112 origin (RFC 7534 §3.2/§5): coordinate first, restrict with an egress filter, or set a private/operator `asn`.

Choose the hand-authored path when these redistribute limits matter: the covering prefixes go to *all* BGP peers because the redistribute block has no dedicated peer-group restriction, the same `asn`/`community` applies to every peer, and the `watchdog` gate checks process serving state rather than the full anycast path.
<!-- source: internal/plugins/as112/redistribute.go -- as112 redistribute producer -->
<!-- source: internal/component/bgp/redistribute/consumer.go -- origin-ASN/community wire emission -->

### Hand-authored origination (per-peer settings)

The `healthcheck` probe plus `watchdog`-controlled `update` block remains available for per-peer origin-AS/community, dedicated peer-group policy, or anycast-path liveness. No AS112-specific BGP code exists; it composes existing mechanisms (see `docs/guide/healthcheck.md`).

### Worked example

```
service {
    as112 {
        enabled true
    }
}

bgp {
    healthcheck {
        probe as112-anycast {
            # H1: query an ANYCAST SERVICE ADDRESS, never loopback. A
            # loopback probe reports UP even when the anycast path itself is
            # unreachable (IP_FREEBIND lets the server bind before the
            # address lands on the interface) -- exactly the false positive
            # RFC 7534 Section 3.3/3.5 forbids.
            #
            # This shells out to `ze cli -c "..."`, which dispatches back
            # into THIS SAME running daemon over SSH. The daemon process
            # itself must be started with ZE_CONFIG_DIR/ZE_SSH_PASSWORD (or
            # an equivalent credential) set in its OWN environment, since the
            # probe subprocess inherits it -- see "Probe credentials" below.
            command "ze cli -c \"request as112 healthcheck target 192.175.48.1\""
            group as112-anycast
            interval 10
            rise 2
            fall 2
            withdraw-on-down true
        }
    }

    # Two DEDICATED peer-groups for AS112 routes only (RFC 7534 Section 3.4's
    # prefix/AS-PATH-filter SHOULD): never reuse a general-purpose/transit
    # peer-group for either update block. `session` is a single container per
    # peer (not a list) -- AS_PATH origin override and "no override" are two
    # SEPARATE peer blocks, both referencing the SAME watchdog group below, so
    # one shared healthcheck probe gates both (see "Shared watchdog groups").
    # Splitting into per-address-family update blocks (rather than one mixed
    # block) is required because next-hop is family-specific.
    #
    # `connection`/`family` blocks are omitted below for brevity -- fill in
    # the usual peer connectivity settings as for any other BGP peer-group.
    # This example (verified with `ze config validate`) focuses only on the
    # AS112-specific composition: the healthcheck-gated watchdog update block.
    peer as112-ix-peer {
        session {
            asn {
                local 65000
                remote 65001
            }
        }
        update {
            attribute {
                origin igp
                next-hop self
                # RFC 7534's recommended community for routes sent to
                # bilateral peers is NOPEER; NO_EXPORT is the other common
                # choice for a local-use mirror. Pick one.
                community [ nopeer ]
            }
            nlri {
                # RFC 7534 Section 3.4 / RFC 7535 Section 2: the covering
                # /24 prefixes -- NOT the /32 host addresses bound on lo
                # (finding H3).
                ipv4/unicast add 192.175.48.0/24
                ipv4/unicast add 192.31.196.0/24
            }
            watchdog {
                name as112-anycast
                # H2: the withdraw marker is MANDATORY. Its absence defaults
                # the route to already-announced (no real YANG default
                # exists despite the misleading description), which would
                # announce the AS112 prefixes before the DNS service is
                # confirmed healthy. `ze doctor` also flags an update block
                # carrying an AS112 covering prefix without this marker
                # (doctor-as112-watchdog-missing-withdraw).
                withdraw true
            }
        }
    }

    # Bilateral peer with an AS112-origin override: asn.local + replace-as
    # overrides AS_PATH origin to 112 for THIS peer-group's routes only. The
    # peer-group above has no such override, so it announces with ze's real
    # local AS instead -- both are controlled independently even though they
    # share the same watchdog group ("as112-anycast").
    #
    # HARD WARNING: replace-as 112 on a group with eBGP sessions to
    # non-private/public ASNs makes this an uncoordinated global AS112 node.
    # Only do this after RFC 7534 Section 3.2/Section 5 coordination (see the
    # umbrella's RFC Compliance Mapping). For a local-use mirror (bilateral/
    # internal peers only), this is safe -- and it is why this example's
    # remote (65002) is a private-use ASN (RFC 6996). `ze doctor` flags
    # asn.local 112 + replace-as against a non-private remote ASN
    # (doctor-as112-global-origin-uncoordinated).
    peer as112-bilateral-peer {
        session {
            asn {
                local 112
                local-options [ replace-as ]
                remote 65002
            }
        }
        update {
            attribute {
                origin igp
                next-hop self
                community [ nopeer ]
            }
            nlri {
                ipv6/unicast add 2620:4f:8000::/48
                ipv6/unicast add 2001:4:112::/48
            }
            watchdog {
                name as112-anycast
                withdraw true
            }
        }
    }
}
```

### Probe credentials

The healthcheck probe's `command` is a plain shell command (`/bin/sh -c "..."`), executed as a child of the `ze` daemon process and inheriting its environment. For `ze cli -c "request as112 healthcheck target ..."` to authenticate back into the same daemon over SSH:

1. Start the daemon with `ZE_CONFIG_DIR` and `ZE_SSH_PASSWORD` (or the super-admin credential of your choice) set in its own environment.
2. Ensure `environment { ssh { enabled true; ... } }` is configured, and a `bgp { ... }` block is present (SSH command dispatch for a healthcheck probe requires the BGP-wired executor path; a bare `ssh {}`-only config with no `bgp {}` block routes through a different, standalone SSH path that does not carry this wiring).
3. Run `ze init` once against that same `ZE_CONFIG_DIR` after the daemon starts, so the client-side credential lookup succeeds on the probe's first tick.

### Shared watchdog groups

A single watchdog group name can be referenced by `update` blocks under multiple distinct peer-groups at the same time. Announce/withdraw state is shared, so sending AS112 routes to two peer-groups needs one `healthcheck` probe and one `group` name, not one per peer-group.

## RFC Compliance Mapping

Every SHOULD/MUST from RFC 7534 (AS112 Nameserver Operations) and RFC 7535 (AS112 Redirection Using DNAME) this feature touches, with an explicit verdict.

| # | Source | Requirement | Verdict |
|---|--------|-------------|---------|
| 1 | RFC 7534 §3.5 | MUST answer authoritatively for each delegated zone | Met. SOA parameters (refresh 1W, retry 1M, expire 1W, min-TTL 1W) and canonical NS/MNAME names are RFC-pinned and tested |
| 2 | RFC 7534 §3.5 | MUST NOT include records beyond SOA/NS in Direct Delegation zones | Met |
| 3 | RFC 7534 §3.5 | MUST NOT host the site's own RFC 1918 records on the AS112 nameserver | Met by design: the plugin only ever serves the fixed static empty-zone data |
| 4 | RFC 7534 §3.3 | SHOULD support cloned loopback / multiple loopback addresses | Met (existing `iface` capability) |
| 5 | RFC 7534 §3.3 | SHOULD dedicate the host to AS112 purpose | **Not met.** Not software-enforceable; deployment recommendation only |
| 6 | RFC 7534 §3.3 | SHOULD order startup: loopback to DNS to routing | Met, conditional on two things: (a) the `update` block includes the `watchdog` `withdraw` marker so the route starts withdrawn (its absence defaults to *announced*); (b) the healthcheck probe queries an anycast service address, not loopback |
| 7 | RFC 7534 §3.3 | SHOULD NOT advertise the service prefix while addresses are unconfigured or DNS is not running | Met (healthcheck → watchdog), same two conditions as #6 |
| 8 | RFC 7534 §3.4 | SHOULD restrict outbound advertisement to a prefix filter permitting only the service prefixes + an AS_PATH filter matching only locally-originated routes | Conditionally met: true if the operator dedicates the target peer-group to AS112 only (recommended above); not enforceable if a general-purpose/transit peer-group is reused |
| 9 | RFC 7534 §3.5 | SHOULD run authoritative-only (recursion disabled) | Met by design |
| 10 | RFC 7534 §3.5 | SHOULD keep HOSTNAME.AS112.{NET,ARPA} TXT answers within 512 octets without EDNS0 | Met. The assembled UDP response (all TXT strings + NS + SOA) is boundary-tested, not any single field in isolation |
| 11 | RFC 7534 §4.1 | SHOULD monitor the node as a production service | Met (Prometheus metrics + `show as112`) |
| 12 | RFC 7534 §4.2 | SHOULD withdraw the service prefix before planned downtime | Met (manual `watchdog withdraw`, or automatic via healthcheck) |
| 13 | RFC 7534 §4.3 | SHOULD measure usage for trend/anomaly tracking | Met (Prometheus counters) |
| 14 | RFC 7534 §3.2/§5 | SHOULD notify the local community before installing; coordinate with other AS112 operators for globally-reachable nodes | **Not met.** Not software-enforceable; process/organisational step |
| 15 | RFC 7534 §3.4 | MAY configure only the relevant address family for single-stack nodes | Met. `address-family` toggle |
| 16 | RFC 7535 §6 | MUST NOT require DNAME support on the AS112 node itself | Met. The plugin only answers EMPTY.AS112.ARPA directly, never processes a DNAME |
| 17 | RFC 7535 §3.1 | SHOULD configure 192.31.196.1/2001:4:112::1 and announce covering routes, and host EMPTY.AS112.ARPA | Met |
| 18 | RFC 7535 §3.1 | SHOULD configure only the relevant address for single-stack nodes | Met. Same toggle as #15 |
| 19 | RFC 7535 §4 | SHOULD leave existing Direct Delegation delegation/continuity unchanged | N/A for local-use deployment (routes never reach the global anycast cloud unless the operator explicitly applies the AS112-origin override on a publicly-peered group, which is then their own coordination responsibility) |

Items #5 and #14 are deployment/process recommendations about how the operator runs and announces the service to the human AS112 community, which Ze cannot verify or enforce in software. They are recorded here, not silently dropped, and repeated below.

## Known Limitations

- NSID (RFC 5001) is not implemented; the `hostname` TXT mechanism is the v1 node-identification approach.
- Only one address per service per family (4 total); BLACKHOLE-1/2 secondary IANA addresses are out of scope.
- Requires in-process deployment because it depends on the iface address-ownership registry's Go-level API, not available to out-of-process/forked plugins.
- `allow-from` drops out-of-range queries silently (no REFUSED response). On-box/loopback is always permitted, so `allow-from` cannot be used to firewall the node from its own healthcheck.
- This plugin ships no executable BGP integration code. Operators hand-author the `healthcheck`/`update` composition from the worked example above; there is no single turnkey `as112 { bgp { ... } }` config block.
- Two RFC 7534 SHOULDs cannot be enforced in software (RFC Compliance Mapping #5 and #14 above): dedicating the host to AS112 purpose, and community notification before installing a globally-reachable node. These are deployment/process responsibilities, documented here rather than coded.
