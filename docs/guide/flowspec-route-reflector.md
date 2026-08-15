# FlowSpec route reflector

Use this when you want one Ze node to receive FlowSpec routes from mitigation systems or edge routers and reflect them to iBGP clients.

The example is a single AS with one Ze route reflector, one upstream FlowSpec receiver, and two edge clients. It enables the `bgp-rr` runtime plugin, negotiates `ipv4/flow`, marks the edge peers as route-reflector clients, and binds the peers to the reflector plugin.

<!-- source: internal/component/bgp/plugins/rr/register.go -- bgp-rr plugin name and dependency -->
<!-- source: internal/component/bgp/plugins/rr/rr.go -- subscriptions and forwarding behavior -->
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- route-reflector-client, cluster-id, next-hop, family -->
<!-- source: internal/component/config/loader.go -- plugin internal use syntax -->
<!-- source: internal/component/bgp/config/plugins.go -- peer process binding validation -->

## 1. Start from an installed Ze node

Follow [Build and install Ze on Ubuntu](ubuntu-build-install.md) through the systemd step. Keep the same active file name, `edge-01.conf`, or replace it in the commands below.

This page uses these addresses:

| Node | Role | IP | AS |
| --- | --- | --- | --- |
| `fs-rr` | Ze FlowSpec route reflector | `10.0.0.1` | `65000` |
| `mitigation-upstream` | upstream or mitigation controller | `10.0.0.2` | `65000` |
| `edge-a` | Ze or router client | `10.0.0.11` | `65000` |
| `edge-b` | Ze or router client | `10.0.0.12` | `65000` |

## 2. Update the active zefs config

Keep the active configuration in `database.zefs`. The commands below read the current active config, normalize it to set format, append the route-reflector settings, render the import file, validate the result, import it back into zefs, and reload the daemon. They do not create `/etc/ze/edge-01.conf`.

```bash
set -euo pipefail

umask 077
CONFIG_SET="$(mktemp)"
CONFIG_IMPORT="$(mktemp)"
trap 'rm -f "$CONFIG_SET" "$CONFIG_IMPORT"' EXIT

sudo /usr/local/bin/ze config cat edge-01.conf | /usr/local/bin/ze config migrate -o "$CONFIG_SET" -

cat >>"$CONFIG_SET" <<'EOF'
set plugin internal bgp-rr use bgp-rr

set bgp router-id 10.0.0.1
set bgp session asn local 65000

set bgp peer mitigation-upstream description "Upstream FlowSpec receiver"
set bgp peer mitigation-upstream connection remote ip 10.0.0.2
set bgp peer mitigation-upstream connection local ip 10.0.0.1
set bgp peer mitigation-upstream connection ttl min 255
set bgp peer mitigation-upstream session asn local 65000
set bgp peer mitigation-upstream session asn remote 65000
set bgp peer mitigation-upstream session route-reflector-client disable
set bgp peer mitigation-upstream session cluster-id 10.0.0.1
set bgp peer mitigation-upstream session next-hop unchanged
set bgp peer mitigation-upstream session family ipv4/flow mode enable
set bgp peer mitigation-upstream session family ipv4/flow prefix maximum 1000
set bgp peer mitigation-upstream attach process bgp-rr receive [ update-received state open-received ]
set bgp peer mitigation-upstream attach process bgp-rr send [ update ]

set bgp peer edge-a description "FlowSpec client edge-a"
set bgp peer edge-a connection remote ip 10.0.0.11
set bgp peer edge-a connection local ip 10.0.0.1
set bgp peer edge-a session asn local 65000
set bgp peer edge-a session asn remote 65000
set bgp peer edge-a session route-reflector-client enable
set bgp peer edge-a session cluster-id 10.0.0.1
set bgp peer edge-a session next-hop unchanged
set bgp peer edge-a session family ipv4/flow mode enable
set bgp peer edge-a session family ipv4/flow prefix maximum 1000
set bgp peer edge-a attach process bgp-rr receive [ update-received state open-received ]
set bgp peer edge-a attach process bgp-rr send [ update ]

set bgp peer edge-b description "FlowSpec client edge-b"
set bgp peer edge-b connection remote ip 10.0.0.12
set bgp peer edge-b connection local ip 10.0.0.1
set bgp peer edge-b session asn local 65000
set bgp peer edge-b session asn remote 65000
set bgp peer edge-b session route-reflector-client enable
set bgp peer edge-b session cluster-id 10.0.0.1
set bgp peer edge-b session next-hop unchanged
set bgp peer edge-b session family ipv4/flow mode enable
set bgp peer edge-b session family ipv4/flow prefix maximum 1000
set bgp peer edge-b attach process bgp-rr receive [ update-received state open-received ]
set bgp peer edge-b attach process bgp-rr send [ update ]
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

What matters:

| Config | Why |
| --- | --- |
| `plugin { internal bgp-rr { use bgp-rr; } }` | Starts the in-process route reflector plugin. |
| `attach process bgp-rr { receive [ update-received state open-received ]; send [ update ]; }` on each peer | Feeds the reflector what it declares it handles, and lets it reflect back. The received direction is not a preference: reflecting an UPDATE raises the sent event the reflection is waiting on, so a plain `update` deadlocks it. |
| `family ipv4/flow` | Negotiates FlowSpec NLRI with the peer. |
| `route-reflector-client enable` on edge clients | Routes from clients are reflected to all clients and non-clients. |
| `next-hop unchanged` | Keeps the mitigation next-hop unchanged. Change this only when your design requires next-hop rewrite. |
| `cluster-id 10.0.0.1` | Uses a stable route reflector cluster ID. |

The plugin dependency `bgp-adj-rib-in` is added by the plugin dependency resolver. There is no separate route-reflector config root.

## 3. Verify sessions and reflector state

```bash
export XDG_RUNTIME_DIR=/run/ze
/usr/local/bin/ze cli -c "show bgp peer list"
/usr/local/bin/ze cli -c "show rr status"
/usr/local/bin/ze cli -c "show rr peers"
```

If a peer does not show the FlowSpec family, check the remote router address-family configuration first. Ze cannot reflect a family that was not negotiated.

## 4. Inject a test FlowSpec rule

The reflector only reflects FlowSpec it *receives*, so originate the test rule from a peer that announces toward the reflector, not from the reflector itself. The rule below matches TCP traffic to `10.0.0.0/8` with destination port 80.

If the source peer (`mitigation-upstream`, `10.0.0.2`) is itself a Ze node, announce toward the reflector (`10.0.0.1`) with the required `peer <selector>` prefix:

```bash
export XDG_RUNTIME_DIR=/run/ze
/usr/local/bin/ze cli -c "peer 10.0.0.1 update text extended-community discard nlri ipv4/flow add destination 10.0.0.0/8 protocol tcp destination-port =80"
```

The `peer <selector>` prefix is required; the update command does not dispatch without it. On a non-Ze source, use that router's FlowSpec origination syntax instead.

Withdraw it with the same match components:

```bash
/usr/local/bin/ze cli -c "peer 10.0.0.1 update text nlri ipv4/flow del destination 10.0.0.0/8 protocol tcp destination-port =80"
```

Then confirm `edge-a` and `edge-b` received the reflected rule with `show rr peers` or the client's own FlowSpec table.

The `bgp-rr` plugin forwards cached UPDATEs and the reactor applies the route-reflection rules, including source exclusion, client to all, non-client to clients only, `ORIGINATOR_ID`, `CLUSTER_LIST`, and next-hop policy.

## 5. Router-side shape

On a conventional router, configure the Ze node as an iBGP route reflector and enable the FlowSpec address family. Vendor syntax differs, but the intent is always the same:

```text
neighbor 10.0.0.1 remote-as 65000
neighbor 10.0.0.1 update-source LOOPBACK0
address-family ipv4 flowspec
  neighbor 10.0.0.1 activate
```

If the router expects TTL security, match the Ze `ttl min 255` policy on the router. If the router uses TCP MD5, add the same `connection md5 password` block to each Ze peer.

## 6. Operations

```bash
export XDG_RUNTIME_DIR=/run/ze
/usr/local/bin/ze cli -c "show rr status"
/usr/local/bin/ze cli -c "show rr peers"
/usr/local/bin/ze cli -c "monitor event"
/usr/local/bin/ze show warnings source bgp
```

Common mistakes:

| Symptom | Check |
| --- | --- |
| Peer up but no FlowSpec routes | Confirm `ipv4/flow` was negotiated on both sides. |
| Updates are reflected back to the origin | Confirm the source peer is bound to `attach process bgp-rr` and is visible in `show rr peers`. |
| Clients do not receive non-client routes | Confirm client peers have `route-reflector-client true`. |
| Config validates but plugin is absent at runtime | Confirm the binary was built with the default plugin set and the `plugin internal bgp-rr` block was imported into the active config. |
