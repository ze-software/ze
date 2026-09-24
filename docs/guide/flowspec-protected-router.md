# FlowSpec protected router

Use this when a Ze router receives BGP FlowSpec rules and should turn them into nftables filters, while also protecting its own BGP control plane from connection floods.

The example enables the nft firewall backend, starts the `flowspec-firewall` bridge plugin, accepts FlowSpec from an iBGP route reflector, and adds a conservative control-plane policing rule for TCP/179.

<!-- source: internal/plugins/flowspec-firewall/register.go -- plugin name and firewall dependency -->
<!-- source: internal/plugins/flowspec-firewall/engine.go -- FlowSpec event subscriptions and firewall apply -->
<!-- source: internal/plugins/flowspec-firewall/state.go -- generated nft table and chain names -->
<!-- source: internal/plugins/copp/yang/ze-copp-conf.yang -- control-plane-protection bgp -->
<!-- source: internal/plugins/ddos/flowspec/yang/ze-ddos-flowspec-conf.yang -- ddos flowspec config -->
<!-- source: docs/guide/firewall.md -- nft firewall backend -->

## 1. Start from an installed Ze node

Follow [Build and install Ze on Ubuntu](ubuntu-build-install.md) through the systemd step. This page assumes `/usr/local/bin/ze`, `/etc/ze`, and active config name `edge-01.conf`.

Example topology:

| Node | Role | IP | AS |
| --- | --- | --- | --- |
| `edge-01` | protected Ze router | `192.0.2.10` | `65010` |
| `flowspec-rr` | FlowSpec route reflector | `203.0.113.1` | `65010` |
| `flowspec-rr-b` | second trusted source | `203.0.113.2` | `65010` |

## 2. Update the active stored config

Keep the active configuration in the `database/` store. The commands below read the current active config, normalize it to set format, append the FlowSpec protection settings, render the import file, validate the result, import it back into the store, and reload the daemon. They do not create `/etc/ze/edge-01.conf`.

```bash
set -euo pipefail

umask 077
CONFIG_SET="$(mktemp)"
CONFIG_IMPORT="$(mktemp)"
trap 'rm -f "$CONFIG_SET" "$CONFIG_IMPORT"' EXIT

sudo /usr/local/bin/ze config cat edge-01.conf | /usr/local/bin/ze config migrate -o "$CONFIG_SET" -

cat >>"$CONFIG_SET" <<'EOF'
set plugin internal flowspec-firewall use flowspec-firewall

set firewall backend nft

set control-plane-protection bgp rate 100/second
set control-plane-protection bgp burst 20
set control-plane-protection bgp protected-port 179
set control-plane-protection bgp trusted-source [ 203.0.113.1/32 203.0.113.2/32 ]
set control-plane-protection bgp over-limit-policy drop

set ddos flowspec response-level enforce
set ddos flowspec action rate-limit
set ddos flowspec rate-limit-bytes 1000000
set ddos flowspec hold-down 300
set ddos flowspec probe-interval 60
set ddos flowspec probe-window 10
set ddos flowspec probe-rate 1000000
set ddos flowspec announce-rate-limit 10
set ddos flowspec max-mitigation-duration 3600
set ddos flowspec backoff-cap 3600
set ddos flowspec blackhole-fallback disable
set ddos flowspec allowlist [ 192.0.2.0/24 2001:db8::/32 ]

set bgp router-id 192.0.2.10
set bgp session asn local 65010

set bgp peer flowspec-rr description "FlowSpec route reflector"
set bgp peer flowspec-rr connection remote ip 203.0.113.1
set bgp peer flowspec-rr connection local ip 192.0.2.10
set bgp peer flowspec-rr connection md5 password change-this-md5-secret
set bgp peer flowspec-rr connection ttl min 255
set bgp peer flowspec-rr session asn local 65010
set bgp peer flowspec-rr session asn remote 65010
set bgp peer flowspec-rr session family ipv4/flow mode enable
set bgp peer flowspec-rr session family ipv4/flow prefix maximum 1000
set bgp peer flowspec-rr session family ipv4/unicast mode enable
EOF

/usr/local/bin/ze config migrate -o "$CONFIG_IMPORT" format hierarchical "$CONFIG_SET"
/usr/local/bin/ze config validate "$CONFIG_IMPORT"
sudo /usr/local/bin/ze config import --name edge-01.conf "$CONFIG_IMPORT"
sudo systemctl reload ze.service
```

Expected validation output:

```text
configuration valid: /tmp/tmp.XXXXXXXXXX
```

What each block does:

| Config | Purpose |
| --- | --- |
| `plugin internal flowspec-firewall` | Starts the bridge and subscribes to validated BGP RIB best-path changes. |
| `firewall backend nft` | Uses Linux nftables as the firewall backend. |
| `control-plane-protection bgp` | Rate-limits new TCP connections to the protected BGP port. |
| `trusted-source` | Bypasses CoPP for known BGP route reflectors or upstream routers. |
| `family ipv4/flow` | Negotiates IPv4 FlowSpec with the route reflector. |
| `ddos flowspec` | Optional automatic upstream mitigation policy for the DDoS pipeline. |

`over-limit-policy drop` is intentionally strict. During first turn-up you can use `accept` to observe without dropping, then switch to `drop` after counters and logs look correct.

The bridge receives selected routes from `bgp-rib`. Received UPDATE events do
not program the firewall. The RIB validates FlowSpec against unicast reachability
and removes rules when authorization is lost. Startup requests replay the
current selected rules, including their action communities.
The same peer must also supply eligible unicast reachability covering the
protected destination, or reflect it with the matching ORIGINATOR_ID. With no
covering unicast route, a received external FlowSpec rule cannot authorize
itself. Enabling the unicast family above supplies that validation input.

The firewall applies an attribute-only replacement to the same rule and removes
the old action even when the replacement cannot be enforced. Native NLRI length
encoding does not create a second rule. VPN FlowSpec remains a BGP validation
and propagation capability; this global firewall bridge installs SAFI 133 only.

At initial configuration, after the firewall backend is ready, the bridge
clears any `ze_flowspec` table left by a previous process before requesting
replay. Live plugin removal suspends selected-route callbacks while it withdraws
the table. A failed reconcile restores the previous desired rules and returns
an error without disabling delivery, so the running configuration remains
usable and removal can be retried. Successful removal stops delivery. Daemon
shutdown remains governed by `firewall { flush-on-shutdown ...; }`; the bridge
does not flush separately.
A compensating Configure after failed removal preserves that live subscription
and selection; it does not repeat the startup sweep. Initialization is complete
only after replay succeeds. A failed initial replay removes its partial rules
and subscription instead of treating that partial view as configured.

<!-- source: internal/plugins/flowspec-firewall/engine.go -- clearStaleRules, removeRules and runEngine -->

<!-- source: internal/plugins/flowspec-firewall/selected.go -- handleSelected -->

Enabling a FlowSpec family derives a `bgp-rib` receive binding for every BGP
peer, including unicast-only peers and dynamic group templates. An explicit
binding must grant `update-received` and `state`; a narrower binding is refused
at configuration load because it would hide routes needed for authorization.
This does not grant the RIB permission to send routes.

FlowSpec authorization requires an internal `bgp-rib`. A `bgp-rs`, `bgp-rr` or
`bgp-adj-rib-in` process that receives a FlowSpec peer's state and has
`send [ update ]` permission must also run internally: its peer-up replay reads
the same process-local selecting RIB. Process aliases do not change this
requirement. An Adj-RIB-In that normally delegates replay still needs access,
because it resumes replay when the forwarder's per-peer ownership is absent.
External processes attached only to unicast peers, or without the state and
UPDATE-send grants needed for replay, remain supported.
Execution mode comes from the effective plugin declaration: `use bgp-rib`
runs in-process even inside an `external` stanza, while
`run "ze plugin bgp-rib"` forks a subprocess. Validation and derived receive
bindings use that same resolved declaration.

<!-- source: internal/component/bgp/config/redistribute_binding.go -- wireRedistributeDelivery -->
<!-- source: internal/component/bgp/config/flowspec_binding.go -- requireRIBDelivery -->

## 3. Check the firewall backend

```bash
export XDG_RUNTIME_DIR=/run/ze
/usr/local/bin/ze cli -c "show bgp peer list"
/usr/local/bin/ze show warnings
sudo nft list ruleset | sed -n '/table inet ze_flowspec/,+80p'
```

The FlowSpec bridge generates an nft table named `ze_flowspec`. The `ze_` prefix is what tells the firewall backend the table is ze's, so a route the peer withdraws takes its rules out of the kernel with it. `show firewall ruleset` and the web pages strip the prefix, so the name you type there stays `flowspec`. The bridge creates base chains for forwarded traffic and local input traffic when rules exist:

| Chain | Hook | Used for |
| --- | --- | --- |
| `flowspec-fwd` | `forward` | Transit packets matching any selected FlowSpec rule. |
| `flowspec-in` | `input` | Locally terminated traffic for addresses owned by the router. |

### What the bridge enforces, and what it refuses

A FlowSpec route becomes a firewall rule only when every one of its components
can be programmed. The IP protocol component is the common limit: the bridge
enforces the ten protocol names the firewall backends know (`icmp`, `tcp`,
`udp`, `gre`, `esp`, `ah`, `icmpv6`, `ospf`, `vrrp`, `sctp`, listed with their
IANA numbers in the firewall guide) and refuses a route naming any other value.

Port components are restricted to TCP and UDP by RFC 8955 Sections 4.2.2.4-6,
including when the rule omits an explicit protocol. SCTP protocol-only rules
can be enforced; SCTP combined with a port component matches no packet and is
refused. ICMP type components select ICMPv4 or ICMPv6 according to the NLRI
family. TCP flag bitmasks are enforced when one masked equality expresses the
whole predicate; unsupported alternatives are refused rather than truncated.

<!-- source: internal/plugins/flowspec-firewall/transport.go -- transportProtocols and tcpFlagsMatch -->
<!-- source: internal/plugins/flowspec-firewall/translate.go -- componentToMatch -->

A refused route is logged with the protocol number and the route key, and
counted in `ze_flowspec_rules_refused_total`. It installs nothing. Ze does not
install the same rule with the protocol condition removed: that rule would drop
more traffic than the peer asked it to drop.

A bridge refusal prevents local firewall installation. The route remains
available to BGP policy and propagation. Other enforceable FlowSpec rules and
the rulesets of other firewall owners continue to reach the kernel.

<!-- source: internal/plugins/flowspec-firewall/translate.go -- protocolMatches -->

### Traffic filtering actions

Ze performs traffic-rate-bytes, traffic-rate-packets, traffic-marking and
traffic-action. The Terminal Action bit has the RFC's counterintuitive meaning:
set continues to later matching rules; clear stops evaluation. Sample enables
packet logging. Rules without an action use normal forwarding.

Rules are ordered by their NLRI components, not by receipt time or peer name.
Each matching rule is sampled and rate-limited once even when both its source-
and destination-port alternatives match. The limiter drops excess traffic;
conforming traffic proceeds according to the Terminal Action bit. Marking is
deferred until all applicable matches have been evaluated against the original
packet header. If several matching rules request different DSCP values, the
highest-precedence marking rule wins. Discard takes precedence over marking
and normal forwarding.

When one route carries several limits in the same unit, the lowest rate wins.
A route asking for both byte and packet limits is refused because the bridge
has one limiter per rule and cannot enforce both dimensions together.

<!-- source: internal/plugins/flowspec-firewall/state.go -- buildTable -->
<!-- source: internal/plugins/flowspec-firewall/rule_chains.go -- ruleChains and markingTerms -->

Known rt-redirect, redirect-to-nexthop and copy-to-nexthop actions remain
unperformable by this bridge. This includes IPv6 next hops carried in the
20-octet IPv6 extended-community attribute, not just ordinary eight-octet
communities. Unknown generic transitive communities have no local packet
action. They remain available to BGP policy and propagation.

<!-- source: internal/plugins/flowspec-firewall/selected.go -- handleSelected -->

RFC 8955 Section 7.4 gives rt-redirect three encodings: a two-octet AS, an IPv4
address and a four-octet AS. RFC 8956 Section 6.1 adds the IPv6-address-specific
route-target redirect with the complete type value `0x000d` in attribute 25.
Ze recognizes and refuses all four; the IPv6 route-target form is distinct
from the subtype `0x0c` redirect-to-next-hop action.

<!-- source: internal/plugins/flowspec-firewall/translate.go -- unperformableAction -->

A route carrying one of those is REFUSED as a whole, logged with the community
that caused the refusal, and counted in `ze_flowspec_rules_refused_total` under
the reason `unsupported-action`. Ze does not install the part of the route it
can perform.

RFC 8955 Section 7 says that where not all traffic filtering actions can be
applied "they should be treated as interfering Traffic Filtering Actions", and
Section 7.7 leaves the choice among interfering actions to the implementation
while asking that the behavior be documented. This section is that document.
The choice is to refuse, because the alternative installs a rule ending in
accept: a route asking ze to rate-limit AND redirect would then send the
traffic to its original destination while the peer was told the route was
accepted.

<!-- source: internal/plugins/flowspec-firewall/translate.go -- unperformableAction -->

## 4. Test with a safe FlowSpec rule

The bridge only programs nftables from FlowSpec that `edge-01` *receives*, so the rule must be announced from a peer toward `edge-01`, not injected on `edge-01` itself. Use a lab prefix first. The example below drops TCP traffic to `10.0.0.0/8` port 80.

The source must already announce eligible unicast reachability covering
`10.0.0.0/8`, as described above. Without that route, the validation gate
refuses the FlowSpec and no firewall rule appears.

If the source (`flowspec-rr`, `203.0.113.1`) is a Ze node, announce toward `edge-01` (`192.0.2.10`) with its peer selector:

```bash
export XDG_RUNTIME_DIR=/run/ze
/usr/local/bin/ze cli -c "send bgp 192.0.2.10 update text extended-community discard nlri ipv4/flow add destination-ipv4 10.0.0.0/8 protocol tcp destination-port =80"
```

On a non-Ze source, use that router's FlowSpec origination syntax instead. Then, on `edge-01`, check that nftables received a rule:

```bash
sudo nft list table inet ze_flowspec
```

Withdraw the test rule from the same source:

```bash
/usr/local/bin/ze cli -c "send bgp 192.0.2.10 update text nlri ipv4/flow del destination-ipv4 10.0.0.0/8 protocol tcp destination-port =80"
```

The rule leaves the kernel with the route. When the withdrawn route was the last
one the peer gave `edge-01`, `nft list ruleset` shows no `ze_flowspec` table at
all. A router upgraded from a build older than 2026-08-23 holds a table named
`flowspec`, without the prefix. Ze removes that one on its first reconcile and
logs `deleting a table an earlier ze build left without the ownership prefix`.

That removal runs when the FlowSpec bridge starts, so it needs the bridge to
still be configured. If you removed the `flowspec-firewall` plugin from the
config in the same upgrade, nothing reconciles the firewall and the old table
keeps enforcing its rules. Check for it and remove it by hand:

```bash
sudo nft list table inet flowspec
sudo nft delete table inet flowspec
```


## 5. Protect the BGP session itself

The example uses three layers:

| Layer | Config |
| --- | --- |
| TCP MD5 | `connection md5 password "..."` |
| TTL security | `connection ttl min 255` |
| CoPP for new sessions | `control-plane-protection bgp` |

Make the remote router match these settings. If the peer is not single-hop, do not use `ttl min 255` without matching the actual TTL design.

## 6. Automatic DDoS FlowSpec policy

The `ddos flowspec` block controls how Ze announces upstream FlowSpec mitigations when the DDoS detection pipeline emits characterized attacks.

| Leaf | Example | Meaning |
| --- | --- | --- |
| `response-level` | `enforce` | Announce mitigation instead of only logging. |
| `action` | `rate-limit` | Announce rate-limit or discard. |
| `rate-limit-bytes` | `1000000` | Bytes per second passed during mitigation. |
| `hold-down` | `300` | Wait before first leak probe. |
| `allowlist` | owned prefixes | Prefixes never announced for mitigation. |

If you only want inbound FlowSpec-to-nftables filtering, keep the `flowspec-firewall` and BGP blocks and remove `ddos flowspec`.

## 7. Common rollback

```bash
sudo /usr/local/bin/ze config history edge-01.conf
sudo /usr/local/bin/ze config rollback 1 edge-01.conf
sudo systemctl reload ze.service
```

Use the revision number from `ze config history`. To stop filtering while keeping
the BGP session, remove the `plugin internal flowspec-firewall` line, validate
the configuration, import it, and reload.
