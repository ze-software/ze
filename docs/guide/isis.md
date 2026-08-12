# IS-IS

Ze implements IS-IS (Intermediate System to Intermediate System, ISO/IEC 10589 as
updated by RFC 1195 / RFC 5305 / RFC 5308 / ...) as a native link-state interior
gateway protocol that runs directly over Layer 2. This guide collects the
user-facing behaviour; the wire format is documented in
[`docs/architecture/wire/isis.md`](../architecture/wire/isis.md).

<!-- This page is the shared IS-IS user guide. Each child spec adds its section;
     keep sections in feature order. -->

> **Interface names are logical.** `interface <name>` in an `isis { }` block refers to
> the Ze *logical* interface name, not necessarily the kernel device name. By default
> the logical name equals the kernel device, but an interface configured with an
> `os-name` selector binds a logical name to a different kernel device; IS-IS resolves
> the name to its kernel device through the shared iface resolver. See
> [Logical Name and the `os-name` Selector](configuration.md#logical-name-and-the-os-name-selector).

<!-- source: internal/plugins/isis/circuits.go -- interfaceIPv4/interfaceIPv6LinkLocal via iface.Addresses -->
<!-- source: internal/plugins/isis/transport/backend_linux.go -- resolveInterface via iface.Resolve -->

## The dynamic hostname

`hostname` names this router in the IS-IS network. Ze floods it in the Dynamic
Hostname TLV 137, and a peer renders it wherever it would otherwise print a
6-octet system ID.

```
isis {
  net 49.0001.0000.0000.0001.00
  hostname core-1.example.net
}
```

**Accepted shape.** RFC 5301 section 3 encodes the value in 7-bit ASCII and calls
its content a domain name. Ze enforces both at the config boundary:

| Rule | Limit |
|------|-------|
| Character set | Printable ASCII only, from space (0x20) to tilde (0x7e) |
| One label | 1 to 63 octets (RFC 2181 section 11) |
| The whole name | 1 to 255 octets, separators included |
| Trailing dot | One is permitted, and it marks the root |

The character rule bounds octets, not vocabulary. RFC 2181 section 11 forbids
restricting WHICH characters a label carries. RFC 5301 section 3 says the value
can be any string an operator wants. So `core_1` and a name with spaces are both
accepted.

**A name outside that shape is REFUSED, not repaired.** `ze config validate` and
a commit both fail, and the error names the offending octet or label:

```
isis: isis/hostname: "café.example" is not a valid IS-IS hostname for isis/hostname:
octet 4 is 0xc3. RFC 5301 section 3 encodes the value in 7-bit ASCII, so each
character must be a printable ASCII character from space (0x20) to tilde (0x7e)
```

Ze does not apply the IDNA ToASCII algorithm to a Unicode name. Converting it
would put `xn--caf-dma.example` on the wire while the config file still read
`café.example`. A name a peer sees must be the name an operator wrote. Use the
ASCII form directly if that is the name you want advertised.

A hostname a PEER advertises is treated as untrusted. It is accepted whatever its
octets, and it is only filtered for display. RFC 5301 section 4 permits that.
Rejecting a peer's LSP over its hostname would hand any neighbor a way to break
this router.

<!-- source: internal/component/config/validators.go -- ISISHostnameValidator -->
<!-- source: internal/plugins/isis/lsdb/encode.go -- hostnameTLV -->
<!-- source: internal/plugins/isis/show.go -- sanitizeHostname -->

## Broadcast LANs: DIS election and pseudo-nodes

On a broadcast (Ethernet, multi-access) circuit, IS-IS does not form a full mesh
of adjacencies. Instead one router per level is elected the **Designated IS**
(DIS), and the LAN is represented as a single virtual node — the **pseudo-node**.

**Election.** Every router advertises a DIS **priority** (0..127, default 64) in
its LAN Hello. The router with the highest priority wins; an equal priority is
broken by the higher interface MAC address. Priority 0 is allowed (it only lowers
preference; it does not stop a router winning on the MAC tiebreak). Set the
priority per interface (and optionally per level) to steer which router becomes the
DIS:

```
isis {
  interfaces {
    interface eth0 {
      circuit-type broadcast
      priority 100        # prefer this router as the LAN DIS
    }
  }
}
```

Election runs **independently for Level 1 and Level 2**, so one router can be the
L1 DIS and another the L2 DIS on the same segment.

**Pseudo-node.** The elected DIS originates a pseudo-node LSP that lists every
router on the segment as a neighbour at metric 0. Every router (the DIS and the
others) then advertises the LAN as a single link to the pseudo-node rather than a
link to each peer, so a LAN with N routers appears in the topology as one node with
N spokes instead of an N*(N-1) mesh. This is automatic; there is nothing to
configure beyond the priority.

**Re-election.** If the DIS leaves the segment (its Hellos stop and the adjacency
times out), the remaining routers elect a new DIS, which originates a fresh
pseudo-node. A router that loses the DIS role while still present withdraws
(purges) its pseudo-node before yielding, so the topology never shows a stale LAN
node. Election is damped so a brief flap on the segment does not churn the
pseudo-node.

**Observability.** Two Prometheus metrics track LAN DIS behaviour:
`ze_isis_dis_elections_total{level}` (DIS elections that changed the elected DIS)
and `ze_isis_pseudonode_lsps{level}` (pseudo-node LSPs this node currently
originates as the DIS). The elected DIS and pseudo-node also appear in the
link-state database (`show isis database`) as an LSP whose ID carries a non-zero
pseudo-node number, e.g. `0000.0000.0001.07-00`.

## Authentication

IS-IS PDUs can be authenticated so a router accepts adjacencies and link-state
information only from peers that share a key. Ze supports three authentication
types, all carried in the IS-IS Authentication TLV (type 10):

| Type | Wire code | Standard | Use |
|------|-----------|----------|-----|
| `cleartext` | 1 | ISO/IEC 10589 | Basic sanity check only — the password rides the wire in clear; not a security mechanism. |
| `hmac-md5` | 54 | RFC 5304 | HMAC-MD5 integrity protection. |
| `hmac-sha-256` (and `hmac-sha-1`/`224`/`384`/`512`) | 3 | RFC 5310 | Generic cryptographic authentication, HMAC-SHA family; SHA-256 recommended. |

**Key chains.** Keys are configured as named `key-chains`, not bare strings. A
chain holds one or more keys, each with a `key-id`, an `algorithm`, a `$9$`-encoded
`secret`, and optional `send-lifetime` / `accept-lifetime` windows for hitless
rotation. The secret is stored with the same reversible `$9$` encoding that PPPoE
and WireGuard keys use, so it never appears as plaintext in `show configuration`,
logs, or backups.

```
isis {
  key-chains area-key {
    key 1 {
      algorithm hmac-sha-256
      secret $9$....               # entered plaintext, stored $9$-encoded on commit
    }
  }
  key-chains iih-key {
    key 1 { algorithm hmac-sha-256  secret ... }
  }
  level-1 { auth-key-chain area-key }     # L1 LSP/CSNP/PSNP: the area key
  level-2 { auth-key-chain domain-key }   # L2 LSP/CSNP/PSNP: the domain key
  interfaces {
    interface eth0 {
      level-1 { auth-key-chain iih-key }    # IIH (Hellos) on this circuit, per level
      level-2 { auth-key-chain iih-key }
    }
  }
}
```

**Per-PDU-class keys.** Hellos (IIH) authenticate with the **per-interface** chain
referenced under `interfaces/interface/<name>/level-N/auth-key-chain` (the Link Level
Authentication string); LSPs and the CSNP/PSNP sequence-number PDUs authenticate
with the **per-level** chain — the *area* key for Level 1 and the *domain* key for
Level 2. The two are independent, so an IIH key never accepts an LSP and vice
versa.

**Hitless rotation.** A chain may hold several keys. The active **signing** key is
the first whose `send-lifetime` is current; on receive, every key whose
`accept-lifetime` is current is tried. Configuring an overlap window (a new key
accepted before it becomes the signing key, the old key accepted for a while after)
lets you roll a key without dropping adjacencies.

**Enforcement.** When a chain is configured for a PDU class, a PDU that arrives
with no Authentication TLV, with the TLV not first, or with a digest no current key
matches is **rejected** before it can form an adjacency, enter the database, or
satisfy synchronisation, and `ze_isis_auth_failures_total{level,interface}` is
incremented. Authenticated **purges** are enforced per RFC 5304: a zero-lifetime
LSP that is unauthenticated, or that carries any TLV other than the Authentication
TLV, is rejected, so a router without the key cannot spoof a purge by zeroing a
captured LSP's lifetime and re-flooding it.

## Redistribution (mesh with BGP)

IS-IS plugs into Ze's protocol-agnostic redistribution framework in both
directions, using the same `redistribute { ... }` block as every other protocol.

**Export IS-IS into BGP.** IS-IS registers a single redistribution source named
`isis`. Importing it into BGP advertises the IS-IS SPF routes (both L1 and L2) as
BGP routes:

```
redistribute {
  destination bgp {
    import isis
  }
}
```

There is one source name `isis` (not per-level `isis-l1`/`isis-l2`): both levels
are exported under it, and the single name is what makes IS-IS self-import
auto-rejected by loop prevention. Per-level redistribution selection is future
work.

**Import other protocols into IS-IS.** An `isis` consumer turns connected, static,
and BGP routes into Extended IP Reachability (TLV 135) entries in the node's own
LSPs, which flood to peers like any other prefix:

```
redistribute {
  destination isis {
    import connected
    import static
    import bgp
  }
}
```

Each redistributed prefix is advertised with a fixed default metric. TLV 135 (IPv4)
carries **no external bit** — a redistributed route is an ordinary reachability
entry; the only wire flag is the up/down bit (RFC 5305 sec 4), which IS-IS sets only
when leaking a prefix to a lower level (RFC 2966), never to mark external origin.
`destination isis { import isis }` is accepted by the schema but is a no-op:
loop prevention rejects redistributing IS-IS into IS-IS.

**Connected and passive interfaces.** IS-IS also advertises its own enabled and
**passive** interface prefixes as internal reachability in its LSPs. A passive
interface forms no adjacency but its prefix is still advertised, so loopbacks and
edge networks are reachable without running the protocol on them.

**Counters.** `ze_isis_redist_injected_total{source,afi}` and
`ze_isis_redist_withdrawn_total{source,afi}` track imported routes;
`ze_isis_redist_inject_failures_total{source}` counts failed re-originations; and
`ze_isis_lsp_reoriginations_total{level}` counts the LSP re-originations
redistribution drives.

Redistribution feeds the redistribute orchestrator only; the kernel FIB install of
IS-IS SPF routes is the separate Loc-RIB path (`AdminDistance` 115). IPv6
redistribution (TLV 236) works the same way (see below).

## Dual-stack IPv6

Ze runs IPv6 over the same IS-IS instance as IPv4 (RFC 5308). Enable the IPv6
address family per interface; both families then ride a **single shared
shortest-path tree** (single-topology), so one adjacency carries both IPv4 and
IPv6 reachability:

```
isis {
  net 49.0001.0000.0000.0001.00
  level l1-l2
  interfaces {
    interface eth0 {
      address-family ipv4-unicast { }
      address-family ipv6-unicast { }   # enable IPv6 on this circuit
    }
  }
}
```

With `ipv6-unicast` enabled on at least one circuit, the node:

- advertises the IPv6 NLPID **0x8E** in the Protocols Supported TLV (129) next to
  IPv4's 0xCC;
- carries its IPv6 **link-local** address in the Hello (IIH) IPv6 Interface
  Address TLV (232), and its **non-link-local** addresses in the LSP TLV 232
  (RFC 5308 sec 3 scope rules), so a dual-stack neighbour learns the IPv6
  next-hop (a link-local `fe80::` address with the egress interface);
- advertises its IPv6 prefixes as IPv6 Reachability (TLV 236) in its own LSP —
  **link-local prefixes are never advertised** in TLV 236 (RFC 5308 sec 2);
- runs an IPv6 route-extraction pass over the **same** Dijkstra tree (no second
  SPF), resolves the IPv6 next-hop, and installs IPv6 routes into the kernel FIB
  through the same Loc-RIB path as IPv4 (`AdminDistance` 115, tagged
  `RTPROT_ze`).

Inspect the installed IPv6 routes with `show isis route ipv6`.

**Wide metrics only.** The TLV 236 prefix metric is a full 32-bit value. A prefix
advertised with a metric greater than `MAX_V6_PATH_METRIC` (`0xFE000000`) is
decoded but excluded from normal SPF (RFC 5308 sec 2), so it never enters the
IPv6 routing table.

**IPv6 redistribution** works in both directions exactly like IPv4, under the same
single `isis` source/consumer: `redistribute { destination bgp { import isis } }`
exports IS-IS IPv6 routes to BGP IPv6 unicast, and `redistribute { destination
isis { import connected/static/bgp } }` imports IPv6 prefixes as TLV 236 entries
(with the external bit set, RFC 5308 sec 2). The redistribution counters carry an
`afi=ipv6` label.

**Single-topology caveat.** Because IPv6 rides the IPv4 SPF tree, Ze assumes the
IPv4 and IPv6 topologies are **congruent** (every link that carries IPv4 also
carries IPv6 with the same metric ordering). On a **non-congruent** topology — a
link that is IPv4-only or IPv6-only, or with per-family metric differences — an
IPv6 route may be computed with a next-hop that has no IPv6 reachability,
blackholing that traffic. The correct fix for non-congruent deployments is RFC
5120 Multi-Topology (a separate per-topology SPF), which Ze does not implement.
For the common congruent dual-stack deployment, single-topology is correct and
simpler.

## Observing and clearing IS-IS

The IS-IS engine exposes its state through `show isis <noun>`, and two runtime
actions reset operational state without touching the configuration.

```
show isis neighbor            # adjacencies: system-id, interface, level, state, hold time
show isis database            # LSDB summary (one row per LSP)
show isis database detail     # LSDB with each LSP's TLVs decoded
show isis route               # IS-IS-computed IPv4 routes
show isis route ipv6          # IS-IS-computed IPv6 routes
show isis interface           # circuits: type, metric, hello/hold, passive, DIS, adjacency count
show isis hostname            # system-id -> dynamic hostname (TLV 137, RFC 5301)
show isis spf-log             # recent SPF runs: time, level, trigger, duration, node count

clear isis adjacency          # tear down all adjacencies; they re-form from the next Hello
clear isis counters           # reset the SPF-run log
```

### Which LSPs are this node's own

Every row of `show isis database` and `show isis database detail` carries an
`own` field. It is `true` on the LSPs this node originated and `false` on the
LSPs it learned from a neighbour. The field is named `own` in the text column and
in the JSON key, so the two forms of the output use one word for it.

Some other implementations mark the same fact with a character beside the LSP ID.
Ze keeps it in a field of its own, because `*` is an input token in ze (the
selector wildcard for "all") and because a marker glued to an identifier does not
survive `| json` or a downstream match on the ID. The `lsp-id` column holds the
LSP ID and nothing else.

Two diagnoses start with this field:

| What you see | What it means |
|--------------|---------------|
| An `own` LSP whose `lifetime` falls toward 0 | The refresh timer is not re-stamping it. At 0 the node disappears from every neighbour's database. |
| An `own` LSP with `purged` set | A neighbour withdrew this node's advertisement, or the refresh failed and the LSP aged out. The daemon logs `isis: own LSP purged` at warn level for the same event. |

Every `show isis ...` command routes its JSON through the pipe machinery, so you
can append `| json`, `| table`, `| text`, `| count`, `| match <pat>`,
`| resolve`, or `| origin`. `clear isis adjacency` forces a full re-learn (LSP
re-origination + SPF re-run) without closing the circuit; `clear isis counters`
clears only the observational SPF-run history (the Prometheus series are
monotonic process counters and are intentionally not reset).

### Web views

The neighbour and database views are also available in the web UI at `/isis` and
`/isis/database`. Each page renders the current snapshot and refreshes live over
Server-Sent Events as adjacencies and the LSDB change.

### Metrics

Every IS-IS subsystem exports Prometheus series under the `ze_isis_*` prefix:
adjacency gauges (`ze_isis_adjacencies_up{level,interface}`,
`ze_isis_adjacencies_total{level}`), LSDB gauges/counters (`ze_isis_lsps`,
`ze_isis_lsp_originations_total`, `ze_isis_purges_total`, ...), flooding/SNP
counters (`ze_isis_lsps_received_total`, `ze_isis_csnp_sent_total`, ...), DIS
(`ze_isis_dis_elections_total`, `ze_isis_pseudonode_lsps`), SPF
(`ze_isis_spf_runs_total`, `ze_isis_spf_duration_seconds`,
`ze_isis_routes_installed{level,afi}`), authentication
(`ze_isis_auth_failures_total{level,interface}`), redistribution
(`ze_isis_redist_injected_total{source,afi}`, ...), and the raw L2 transport
(`ze_isis_frames_sent_total{interface}`, `ze_isis_sockets_open`). The full set
with labels is listed in `docs/plugin-development/metrics.md`.

### Readiness checks

`ze doctor` reports IS-IS readiness problems before the engine starts:

- `doctor-isis-raw-socket` — IS-IS is configured but a raw `AF_PACKET` socket
  cannot be opened (needs `CAP_NET_RAW`); IS-IS forms no adjacencies without it.
- `doctor-isis-net-missing` — the `isis` block is present but no `net` is set;
  IS-IS cannot derive a System ID or originate LSPs.
- `doctor-isis-system-id-mismatch` — an explicit `system-id` disagrees with the
  System ID embedded in the first NET.

Each code is explainable with `ze explain <code>`.
