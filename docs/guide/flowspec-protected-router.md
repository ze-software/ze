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

## 2. Write the config

```bash
sudo tee /etc/ze/edge-01.conf >/dev/null <<'EOF'
plugin {
    internal flowspec-firewall {
        use flowspec-firewall
    }
}

firewall {
    backend nft
}

control-plane-protection {
    bgp {
        rate 100/second
        burst 20
        protected-port [ 179 ]
        trusted-source [ 203.0.113.1/32 203.0.113.2/32 ]
        over-limit-policy drop
    }
}

ddos {
    flowspec {
        response-level enforce
        action rate-limit
        rate-limit-bytes 1000000
        hold-down 300
        probe-interval 60
        probe-window 10
        probe-rate 1000000
        announce-rate-limit 10
        max-mitigation-duration 3600
        backoff-cap 3600
        blackhole-fallback false
        allowlist [ 192.0.2.0/24 2001:db8::/32 ]
    }
}

bgp {
    router-id 192.0.2.10
    session {
        asn {
            local 65010
        }
    }

    peer flowspec-rr {
        description "FlowSpec route reflector"
        connection {
            remote {
                ip 203.0.113.1
            }
            local {
                ip 192.0.2.10
            }
            md5 {
                password "change-this-md5-secret"
            }
            ttl {
                min 255
            }
        }
        session {
            asn {
                local 65010
                remote 65010
            }
            family {
                ipv4/flow {
                    mode enable
                    prefix {
                        maximum 1000
                    }
                }
            }
        }
        process flowspec-firewall {
        }
    }
}
EOF
sudo chmod 0600 /etc/ze/edge-01.conf
```

What each block does:

| Config | Purpose |
| --- | --- |
| `plugin internal flowspec-firewall` | Starts the in-process bridge that listens for FlowSpec UPDATE events. |
| `firewall backend nft` | Uses Linux nftables as the firewall backend. |
| `control-plane-protection bgp` | Rate-limits new TCP connections to the protected BGP port. |
| `trusted-source` | Bypasses CoPP for known BGP route reflectors or upstream routers. |
| `family ipv4/flow` | Negotiates IPv4 FlowSpec with the route reflector. |
| `process flowspec-firewall` | Binds this peer to the bridge plugin. |
| `ddos flowspec` | Optional automatic upstream mitigation policy for the DDoS pipeline. |

`over-limit-policy drop` is intentionally strict. During first turn-up you can use `accept` to observe without dropping, then switch to `drop` after counters and logs look correct.

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

## 4. Check the firewall backend

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

## 5. Test with a safe FlowSpec rule

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

## 6. Protect the BGP session itself

The example uses three layers:

| Layer | Config |
| --- | --- |
| TCP MD5 | `connection md5 password "..."` |
| TTL security | `connection ttl min 255` |
| CoPP for new sessions | `control-plane-protection bgp` |

Make the remote router match these settings. If the peer is not single-hop, do not use `ttl min 255` without matching the actual TTL design.

## 7. Automatic DDoS FlowSpec policy

The `ddos flowspec` block controls how Ze announces upstream FlowSpec mitigations when the DDoS detection pipeline emits characterized attacks.

| Leaf | Example | Meaning |
| --- | --- | --- |
| `response-level` | `enforce` | Announce mitigation instead of only logging. |
| `action` | `rate-limit` | Announce rate-limit or discard. |
| `rate-limit-bytes` | `1000000` | Bytes per second passed during mitigation. |
| `hold-down` | `300` | Wait before first leak probe. |
| `allowlist` | owned prefixes | Prefixes never announced for mitigation. |

If you only want inbound FlowSpec-to-nftables filtering, keep the `flowspec-firewall` and BGP blocks and remove `ddos flowspec`.

## 8. Common rollback

```bash
sudo cp /etc/ze/edge-01.conf /etc/ze/edge-01.conf.before-flowspec
sudo /usr/local/bin/ze config validate /etc/ze/edge-01.conf
sudo systemctl reload ze.service
```

To stop enforcing received FlowSpec while keeping the BGP session, remove the `process flowspec-firewall` binding and the `plugin internal flowspec-firewall` block, validate, import, and reload.
