# Control-Plane Policing for BGP

A DDoS aimed at the router's own address, a TCP/179 connection flood, can
saturate the CPU, starve BGP keepalive processing, and stop the router from
sending the FlowSpec and RTBH signals that would clear it. The firewall component
had the parts (input-hook chains, the `Limit` action, a destination-port match,
the nft backend) and no construct that protects host-bound BGP traffic.

The `copp` plugin generates a managed nft input chain.

## Decisions

### A system plugin, not a firewall extension

<!-- source: internal/plugins/copp/register.go -- plugin registration, lifecycle -->

CoPP is domain policy over the firewall datapath. Removing the plugin directory
must remove all of CoPP, which is what plugin self-containment requires.

It mirrors the `policyroute` pattern, `RegisterTables` plus `ApplyAll`, rather
than adding a firewall API. The existing registry already handles several owners
coexisting on one table set. The rules that keep those owners' tables visible and
reconciled are in
[firewall table ownership](../firewall/table-ownership-and-shutdown-flush.md).

### The neighbors come from the bgp root copp reads

<!-- source: internal/plugins/copp/register.go -- bgpConfigRoot, ConfigReads -->
<!-- source: internal/plugins/copp/config.go -- parseCoppSections -->
<!-- source: internal/component/config/bgp_neighbor_sources.go -- BGPNeighborSources -->

RFC 7454 Section 4 asks for an ACL that discards traffic to port 179 "sourced
from an address not known or permitted to become a BGP neighbor". copp learns
those addresses from the configuration, never from the reactor, so it works the
same when it runs in another process. It lists `bgp` under `ConfigReads`, not
`ConfigRoots`: the engine delivers the `bgp` section beside
`control-plane-protection` at start and whole on every reload that changes
either root, and a configuration that holds only `bgp` does not load copp, so
CoPP stays opt-in.

The section is read by `config.BGPNeighborSources`, the one reader of a peer's
`connection > remote` container, which the BGP config resolve and the
reactor's peer parse also call. Each static peer becomes a host prefix of its
remote address, and each dynamic group contributes its `range` entries. A `bgp`
section the BGP parse would refuse refuses the CoPP policy too, because a set
built from part of it would drop a configured neighbor. The reader lives in the
config component because copp (`ze_copp`) does not depend on BGP (`ze_bgp`).

### The chain policy is always accept

<!-- source: internal/plugins/copp/translate.go -- chain policy, term order -->

A base input chain whose policy is `drop` discards every packet that reaches the
end of the chain. The CoPP terms name TCP to the protected ports and nothing
else. Such a policy would take SSH, ICMP and every other service with it. The
operator would lose the box on the first apply. The policy is therefore `accept`
under both settings of `over-limit-policy`.

The drop that the ACL needs is a term instead, and it names TCP to a protected
port only.

### The ACL: neighbors in, every other source out

<!-- source: internal/plugins/copp/translate.go -- translatePolicy, neighborChainTerms, intervalElements -->
<!-- source: internal/plugins/copp/config.go -- parseRateLimit -->

The `input` base chain holds, in this order:

| Term | Matches | Action |
|------|---------|--------|
| `established` | connection state established or related | accept |
| `trusted-<prefix>` | TCP to a protected port from one `trusted-source` prefix (`/` and `:` written as `-`) | accept |
| `bgp-neighbor-ipv4`, `bgp-neighbor-ipv6` | TCP to a protected port from a source in that family's neighbor set | jump to `bgp-neighbor` |
| `unknown-source-drop` | TCP to a protected port, any source, any state | count, drop |

The neighbor sets are interval sets named `bgp-neighbor-ipv4` and
`bgp-neighbor-ipv6`, one per family that has a neighbor. A static peer inside a
dynamic group's range is folded into the range, because the kernel refuses
overlapping intervals. A family with no neighbor gets no set and no term.

The regular chain `bgp-neighbor` holds the limiter, when `rate` is set, then an
accept. One limiter serves both families, so `rate` is one budget for all
neighbors, as the operator wrote it. Only a source the input chain found in a
neighbor set reaches it, so a flood from an unknown source is dropped before it
can spend the neighbors' budget (RFC 7454 Section 4: "In addition to strict
filtering, rate-limiting MAY be configured for accepted BGP traffic."). A
`trusted-source` never meets the limiter.

`rate` is optional. Without it the table carries the ACL and no limiter.
`over-limit-policy` rides the limiter, as the nftables limiter and its inversion
flag. `drop`, the default, emits `limit rate over N` and then `drop`: that rule
matches only once the bucket is empty, so an over-limit connection is dropped
and an under-limit one falls through to the accept. `accept` emits
`limit rate N` and then `accept`: the over-limit connection falls through to
the same accept, so the limit reports load and stops nothing. Neither setting
changes the ACL.

The drop catches every connection state, because an unknown source cannot own
an established session: the `established` term has already accepted every live
one. A `drop` setting never closes a session that is already up.

### Term order is fixed by construction

The append sequence is established, trusted, neighbors, drop. It is not
configurable, because the wrong order is the dangerous failure mode and an
operator has no reason to want a different one.

### CoPP stays opt-in

RFC 7454 Section 4 notes that "Some routers automatically program such an ACL
upon BGP configuration." Ze does not: a configuration without
`control-plane-protection bgp` gets no ACL, so an upgrade never adds an
implicit deny to a deployment that did not ask for one.

### One dual-stack table

<!-- source: internal/plugins/copp/model.go -- coppPolicy -->

`FamilyInet` covers IPv4 and IPv6 in one input chain. Separate `ip` and `ip6`
tables would double the state for no behavioral difference.

### `ParseRateSpec` is exported from the firewall package

<!-- source: internal/component/firewall/config.go -- ParseRateSpec -->

copp needs the same rate-spec parsing the firewall config implements. Exporting
it beat duplicating the parser. It is now public API of the firewall package and
any plugin with a rate-spec leaf can use it.

## Consequences

<!-- source: internal/plugins/copp/doctor.go -- checkCoppInputChain, expectedTable, missingFromKernel -->

- Any future control-plane protection (OSPF, LDP, SSH) extends this plugin with
  another protocol block under `control-plane-protection { ... }`.
- The copp table uses priority 0, the standard filter priority. A drop in any
  base chain at a hook is final, so the `unknown-source-drop` term stands
  wherever the operator's own input chains sit.
- The `copp-input-chain` doctor check (code `doctor-copp-missing`) builds the
  table this configuration asks for with `translatePolicy`, reads the kernel's
  tables back through the firewall backend, and names each part the kernel does
  not hold: the whole table, a chain, a term by name, or a set. It names
  `ze signal restart` as the fix, because a reload that changes neither root
  copp reads does not reach copp. When the kernel tables cannot be read, it
  says so rather than reporting the table present.
