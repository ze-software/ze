# FlowSpec route reflector

Use this when you want one Ze node to receive FlowSpec routes from mitigation systems or edge routers and reflect them to iBGP clients.

The example is a single AS with one Ze route reflector, one upstream FlowSpec receiver, and two edge clients. It enables the `bgp-rr` runtime plugin, negotiates `ipv4/flow`, marks the edge peers as route-reflector clients, and binds the peers to the reflector plugin.

<!-- source: internal/component/bgp/plugins/rr/register.go -- bgp-rr plugin name and dependency -->
<!-- source: internal/component/bgp/plugins/rr/rr.go -- subscriptions and forwarding behavior -->
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- route-reflector-client, cluster-id, next-hop, family -->
<!-- source: internal/component/config/loader.go -- plugin internal use syntax -->
<!-- source: internal/component/bgp/config/plugins.go -- peer process binding validation -->

## 1. Start from an installed Ze node

Follow [Build and install Ze on Ubuntu](../ubuntu-build-install/index.md) through the systemd step. Keep the same active file name, `edge-01.conf`, or replace it in the commands below.

This page uses these addresses:

| Node | Role | IP | AS |
| --- | --- | --- | --- |
| `fs-rr` | Ze FlowSpec route reflector | `10.0.0.1` | `65000` |
| `mitigation-upstream` | upstream or mitigation controller | `10.0.0.2` | `65000` |
| `edge-a` | Ze or router client | `10.0.0.11` | `65000` |
| `edge-b` | Ze or router client | `10.0.0.12` | `65000` |

## 2. Write the Ze config

```bash
sudo tee /etc/ze/edge-01.conf >/dev/null <<'EOF'
plugin {
    internal bgp-rr {
        use bgp-rr
    }
}

bgp {
    router-id 10.0.0.1
    session {
        asn {
            local 65000
        }
    }

    peer mitigation-upstream {
        description "Upstream FlowSpec receiver"
        connection {
            remote {
                ip 10.0.0.2
            }
            local {
                ip 10.0.0.1
            }
            ttl {
                min 255
            }
        }
        session {
            asn {
                local 65000
                remote 65000
            }
            route-reflector-client false
            cluster-id 10.0.0.1
            next-hop unchanged
            family {
                ipv4/flow {
                    mode enable
                    prefix {
                        maximum 1000
                    }
                }
            }
        }
        process bgp-rr {
        }
    }

    peer edge-a {
        description "FlowSpec client edge-a"
        connection {
            remote {
                ip 10.0.0.11
            }
            local {
                ip 10.0.0.1
            }
        }
        session {
            asn {
                local 65000
                remote 65000
            }
            route-reflector-client true
            cluster-id 10.0.0.1
            next-hop unchanged
            family {
                ipv4/flow {
                    mode enable
                    prefix {
                        maximum 1000
                    }
                }
            }
        }
        process bgp-rr {
        }
    }

    peer edge-b {
        description "FlowSpec client edge-b"
        connection {
            remote {
                ip 10.0.0.12
            }
            local {
                ip 10.0.0.1
            }
        }
        session {
            asn {
                local 65000
                remote 65000
            }
            route-reflector-client true
            cluster-id 10.0.0.1
            next-hop unchanged
            family {
                ipv4/flow {
                    mode enable
                    prefix {
                        maximum 1000
                    }
                }
            }
        }
        process bgp-rr {
        }
    }
}
EOF
sudo chmod 0600 /etc/ze/edge-01.conf
```

What matters:

| Config | Why |
| --- | --- |
| `plugin internal bgp-rr { use bgp-rr }` | Starts the in-process route reflector plugin. |
| `process bgp-rr { }` on each peer | Binds the peer to the declared plugin name. |
| `family ipv4/flow` | Negotiates FlowSpec NLRI with the peer. |
| `route-reflector-client true` on edge clients | Routes from clients are reflected to all clients and non-clients. |
| `next-hop unchanged` | Keeps the mitigation next-hop unchanged. Change this only when your design requires next-hop rewrite. |
| `cluster-id 10.0.0.1` | Uses a stable route reflector cluster ID. |

The plugin dependency `bgp-adj-rib-in` is added by the plugin dependency resolver. There is no separate route-reflector config root.

## 3. Validate, import, and reload

```bash
sudo /usr/local/bin/ze config validate /etc/ze/edge-01.conf
sudo /usr/local/bin/ze config import --name edge-01.conf /etc/ze/edge-01.conf
sudo systemctl reload ze.service
```

Expected validation output:

```text
configuration valid: /etc/ze/edge-01.conf
```

## 4. Verify sessions and reflector state

```bash
export XDG_RUNTIME_DIR=/run/ze
/usr/local/bin/ze cli -c "show bgp peer list"
/usr/local/bin/ze cli -c "show rr status"
/usr/local/bin/ze cli -c "show rr peers"
```

If a peer does not show the FlowSpec family, check the remote router address-family configuration first. Ze cannot reflect a family that was not negotiated.

## 5. Inject a test FlowSpec rule

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

## 6. Router-side shape

On a conventional router, configure the Ze node as an iBGP route reflector and enable the FlowSpec address family. Vendor syntax differs, but the intent is always the same:

```text
neighbor 10.0.0.1 remote-as 65000
neighbor 10.0.0.1 update-source LOOPBACK0
address-family ipv4 flowspec
  neighbor 10.0.0.1 activate
```

If the router expects TTL security, match the Ze `ttl min 255` policy on the router. If the router uses TCP MD5, add the same `connection md5 password` block to each Ze peer.

## 7. Operations

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
| Updates are reflected back to the origin | Confirm the source peer is bound to `process bgp-rr` and is visible in `show rr peers`. |
| Clients do not receive non-client routes | Confirm client peers have `route-reflector-client true`. |
| Config validates but plugin is absent at runtime | Confirm the binary was built with the default plugin set and the `plugin internal bgp-rr` block was imported into the active config. |
