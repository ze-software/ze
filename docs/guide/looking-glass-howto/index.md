# Public looking glass

Use this when you want a read-only HTTP looking glass for BGP peers, route lookup, prefix search, and AS-path graphs.

The service is public and has no authentication by design. Put it on an address where that is acceptable, or publish it through a reverse proxy that provides the access control you need.

<!-- source: internal/component/lg/yang/ze-lg-conf.yang -- environment looking-glass config -->
<!-- source: internal/component/lg/server.go -- looking glass HTTP routes -->
<!-- source: docs/features/looking-glass.md -- feature behavior and public read-only note -->
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- BGP peer family config -->

## 1. Start from an installed Ze node

Follow [Build and install Ze on Ubuntu](../ubuntu-build-install/index.md) through the systemd step. This page assumes `/usr/local/bin/ze`, `/etc/ze`, and active config name `edge-01.conf`.

Example topology:

| Node | Role | IP | AS |
| --- | --- | --- | --- |
| `lg-01` | Ze looking glass | `198.51.100.10` | `65020` |
| `rs1` | route server or upstream peer | `198.51.100.1` | `65000` |
| public HTTP | looking glass | `0.0.0.0:8443` | n/a |
| local SSH | operator CLI | `127.0.0.1:2222` | n/a |

## 2. Write the config

```bash
ADMIN_HASH="$(printf '%s\n' 'CHANGE_ME_BOOTSTRAP' | /usr/local/bin/ze passwd)"

sudo tee /etc/ze/edge-01.conf >/dev/null <<EOF
environment {
    looking-glass {
        enabled true
        server public {
            ip 0.0.0.0
            port 8443
        }
        tls false
    }
    ssh {
        enabled true
        server ops {
            ip 127.0.0.1
            port 2222
        }
    }
}

system {
    authentication {
        user admin {
            password "$ADMIN_HASH"
            profile [ admin ]
        }
    }
    authorization {
        profile admin {
            run {
                default-action allow
            }
            edit {
                default-action allow
            }
        }
    }
}

bgp {
    router-id 198.51.100.10
    session {
        asn {
            local 65020
        }
    }

    peer rs1 {
        description "Route server peer"
        connection {
            remote {
                ip 198.51.100.1
            }
            local {
                ip 198.51.100.10
            }
        }
        session {
            asn {
                local 65020
                remote 65000
            }
            family {
                ipv4/unicast {
                    prefix {
                        maximum 1000000
                    }
                }
            }
        }
    }
}
EOF
sudo chmod 0600 /etc/ze/edge-01.conf
```

The looking glass server defaults are `ip 0.0.0.0`, `port 8443`, and `tls false`; the values are shown explicitly so the copy-paste config is clear.

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

## 4. Open the UI

From a browser:

```text
http://198.51.100.10:8443/lg/peers
http://198.51.100.10:8443/lg/search
http://198.51.100.10:8443/lg/graph?prefix=10.10.1.0/24&mode=aspath
```

For a local test on the box:

```bash
curl -fsS http://127.0.0.1:8443/api/looking-glass/status
curl -fsS http://127.0.0.1:8443/api/looking-glass/protocols/bgp
curl -fsS 'http://127.0.0.1:8443/api/looking-glass/routes/prefix?prefix=10.0.0.0/24'
```

## 5. Birdwatcher-compatible API paths

The looking glass exposes birdwatcher-style read-only endpoints under `/api/looking-glass/`.

| Path | Purpose |
| --- | --- |
| `/api/looking-glass/status` | service and router status |
| `/api/looking-glass/protocols/bgp` | BGP peer table |
| `/api/looking-glass/protocols/short` | compact protocol list |
| `/api/looking-glass/routes/protocol/{name}` | routes for one protocol or peer |
| `/api/looking-glass/routes/peer/{peer}` | routes learned from one peer |
| `/api/looking-glass/routes/table/{family}` | routes for a family table |
| `/api/looking-glass/routes/prefix?prefix=10.0.0.0/24` | exact or containing prefix lookup |
| `/api/looking-glass/routes/search?prefix=10.0.0.0/24` | prefix search |

Additional endpoints exist for filtered, exported, and no-export routes, route counts, and BMP-derived tables.

The UI under `/lg/` uses the same data and adds the peer table, route lookup, route search, and AS-path graph.

## 6. Publish it safely

The looking glass is read-only, but it still publishes topology and routing information. Common deployment choices:

| Deployment | Config |
| --- | --- |
| Public service directly on Ze | `ip 0.0.0.0`, firewall permits TCP/8443 |
| Reverse proxy on the same host | `ip 127.0.0.1`, proxy handles TLS and policy |
| Internal only | bind to a management address and filter at the network edge |

If Ze terminates TLS itself, set `tls true`. This requires a zefs blob store to hold the certificate, so create one first with `ze init` (see [Build and install Ze on Ubuntu](../ubuntu-build-install/index.md)); otherwise startup fails with `looking glass TLS requires blob storage (run ze init first)`.

```text
environment {
    looking-glass {
        enabled true
        server public {
            ip 0.0.0.0
            port 8443
        }
        tls true
    }
}
```

## 7. Operations

```bash
export XDG_RUNTIME_DIR=/run/ze
/usr/local/bin/ze cli -c "show bgp peer list"
/usr/local/bin/ze show warnings
journalctl -u ze.service -n 100 --no-pager
```

If the UI is empty, check that BGP peers are established and that the relevant families are configured. The looking glass can only show routes Ze has learned or originated.
