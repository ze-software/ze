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

Follow [Build and install Ze on Ubuntu](../ubuntu-build-install/index.md) through the systemd step. This page assumes `/usr/local/bin/ze`, `/etc/ze`, and active config name `edge-01.conf`.

Example topology:

| Node | Role | IP | AS |
| --- | --- | --- | --- |
| `edge-01` | protected Ze router | `192.0.2.10` | `65010` |
| `flowspec-rr` | FlowSpec route reflector | `203.0.113.1` | `65010` |
| `flowspec-rr-b` | second trusted source | `203.0.113.2` | `65010` |

## 2. Update the active zefs config

Keep the active configuration in `database.zefs`. The commands below read the current active config, normalize it to set format, append the FlowSpec protection settings, render the import file, validate the result, import it back into zefs, and reload the daemon. They do not create `/etc/ze/edge-01.conf`.

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
set bgp peer flowspec-rr attach process flowspec-firewall receive [ update-received state ]
EOF

/usr/local/bin/ze config migrate --format hierarchical -o "$CONFIG_IMPORT" "$CONFIG_SET"
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
| `plugin internal flowspec-firewall` | Starts the in-process bridge that listens for FlowSpec UPDATE events. |
| `firewall backend nft` | Uses Linux nftables as the firewall backend. |
| `control-plane-protection bgp` | Rate-limits new TCP connections to the protected BGP port. |
| `trusted-source` | Bypasses CoPP for known BGP route reflectors or upstream routers. |
| `family ipv4/flow` | Negotiates IPv4 FlowSpec with the route reflector. |
| `attach process flowspec-firewall { receive [ update-received state ]; }` | Feeds the plugin the FlowSpec routes this peer announces, and its session state. Without the receive list the peer feeds it nothing. |
| `ddos flowspec` | Optional automatic upstream mitigation policy for the DDoS pipeline. |

`over-limit-policy drop` is intentionally strict. During first turn-up you can use `accept` to observe without dropping, then switch to `drop` after counters and logs look correct.

## 3. Check the firewall backend

```bash
export XDG_RUNTIME_DIR=/run/ze
/usr/local/bin/ze cli -c "show bgp peer list"
/usr/local/bin/ze show warnings
sudo nft list ruleset | sed -n '/table inet flowspec/,+80p'
```

The FlowSpec bridge generates an nft table named `flowspec`. It creates base chains for forwarded traffic and local input traffic when rules exist:

| Chain | Hook | Used for |
| --- | --- | --- |
| `flowspec-fwd` | `forward` | Transit traffic that matches non-local FlowSpec destinations. |
| `flowspec-in` | `input` | Locally terminated traffic for addresses owned by the router. |

## 4. Test with a safe FlowSpec rule

The bridge only programs nftables from FlowSpec that `edge-01` *receives*, so the rule must be announced from a peer toward `edge-01`, not injected on `edge-01` itself. Use a lab prefix first. The example below drops TCP traffic to `10.0.0.0/8` port 80.

If the source (`flowspec-rr`, `203.0.113.1`) is a Ze node, announce toward `edge-01` (`192.0.2.10`) with the required `peer <selector>` prefix:

```bash
export XDG_RUNTIME_DIR=/run/ze
/usr/local/bin/ze cli -c "peer 192.0.2.10 update text extended-community discard nlri ipv4/flow add destination 10.0.0.0/8 protocol tcp destination-port =80"
```

On a non-Ze source, use that router's FlowSpec origination syntax instead. Then, on `edge-01`, check that nftables received a rule:

```bash
sudo nft list table inet flowspec
```

Withdraw the test rule from the same source:

```bash
/usr/local/bin/ze cli -c "peer 192.0.2.10 update text nlri ipv4/flow del destination 10.0.0.0/8 protocol tcp destination-port =80"
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

Use the revision number from `ze config history`. To stop enforcing received FlowSpec while keeping the BGP session, remove the `attach process flowspec-firewall` binding and the `plugin internal flowspec-firewall` line with the same zefs update pattern, validate, import, and reload.
