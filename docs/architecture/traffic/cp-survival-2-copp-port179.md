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

### The chain policy is always accept

<!-- source: internal/plugins/copp/translate.go -- chain policy, term order -->

A base input chain whose policy is `drop` discards every packet that reaches the
end of the chain. The CoPP terms name TCP to the protected ports and nothing
else. Such a policy would take SSH, ICMP and every other service with it. The
operator would lose the box on the first apply. The policy is therefore `accept`
under both settings of `over-limit-policy`.

`over-limit-policy` rides the rate-limit term instead, as the nftables limiter
and its inversion flag. `accept` emits `limit rate N` and then `accept`. That
rule matches while the token bucket has credit, so an over-limit packet falls
through to the chain policy and is accepted. `drop` emits `limit rate over N`
and then `drop`. That rule matches only once the bucket is empty, so an
over-limit packet is dropped. Every under-limit packet falls through to the same
accept policy.

An established session is accepted by the first term, before the limiter is
reached. A `drop` setting never closes a session that is already up.

### Term order is fixed by construction

The append sequence is established, then trusted, then limit. It is not
configurable, because the wrong order is the dangerous failure mode and an
operator has no reason to want a different one.

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

<!-- source: internal/plugins/copp/doctor.go -- doctor-copp-missing -->

- Any future control-plane protection (OSPF, LDP, SSH) extends this plugin with
  another protocol block under `control-plane-protection { ... }`.
- The copp table uses priority 0, the standard filter priority. When an
  operator's own input chain also uses priority 0, evaluation order depends on
  table creation order. The doctor check warns when CoPP is configured and the
  chain may not be active.
