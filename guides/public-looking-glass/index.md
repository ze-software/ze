# Public looking glass

Use this when you want a read-only HTTP looking glass for BGP peers, route lookup, prefix search, and AS-path graphs.

The service is read-only, and open by default because a looking glass is a public surface. Put it on an address where that is acceptable, or publish it through a reverse proxy that provides the access control you need. To gate it yourself, set `environment looking-glass token <secret>`: every `/api/` and `/lg/` route then needs `Authorization: Bearer <secret>`, and a request without it gets `401`.

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

## 2. Update the active zefs config

Keep the active configuration in `database.zefs`. The commands below read the current active config, normalize it to set format, append the looking-glass settings, render the import file, validate the result, import it back into zefs, and reload the daemon. They do not create `/etc/ze/edge-01.conf`.

```bash
set -euo pipefail

umask 077
CONFIG_SET="$(mktemp)"
CONFIG_IMPORT="$(mktemp)"
trap 'rm -f "$CONFIG_SET" "$CONFIG_IMPORT"' EXIT

sudo /usr/local/bin/ze config cat edge-01.conf | /usr/local/bin/ze config migrate -o "$CONFIG_SET" -

cat >>"$CONFIG_SET" <<'EOF'
set environment looking-glass enabled enable
set environment looking-glass server public ip 0.0.0.0
set environment looking-glass server public port 8443
set environment looking-glass tls false

set environment ssh server main ip 127.0.0.1
set environment ssh server main port 2222

set bgp router-id 198.51.100.10
set bgp session asn local 65020
set bgp peer rs1 description "Route server peer"
set bgp peer rs1 connection remote ip 198.51.100.1
set bgp peer rs1 connection local ip 198.51.100.10
set bgp peer rs1 session asn local 65020
set bgp peer rs1 session asn remote 65000
set bgp peer rs1 session family ipv4/unicast prefix maximum 1000000
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

The looking glass server defaults are `ip 0.0.0.0`, `port 8443`, and `tls true`. This example sets `tls false` because it puts a reverse proxy in front; the values are shown explicitly so the copy-paste config is clear. The example keeps SSH on localhost by updating the existing `server main` listener from the Ubuntu install guide.

## 3. Open the UI

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

## 4. Birdwatcher-compatible API paths

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

The BGP protocol entry is keyed by the configured peer name. It includes the
last state transition, the last session error, and live route counts:

```json
{
  "protocols": {
    "rs1": {
      "state": "established",
      "state_changed": "2026-07-15T10:30:00Z",
      "last_error": "",
      "routes_received": 600,
      "routes_imported": 600,
      "routes_exported": 580,
      "routes_filtered": 0
    }
  }
}
```

`routes_received` and `routes_imported` both report the retained Adj-RIB-In
size because Ze does not keep routes rejected by import policy.
`routes_exported` is the Adj-RIB-Out size. `routes_filtered` remains zero for
the same reason, and the filtered-routes endpoint returns an empty list. Route
counts are present only when the `bgp-rib` plugin is loaded.

<!-- source: internal/component/lg/handler_api.go -- transformProtocols -->
<!-- source: internal/component/bgp/plugins/cmd/peer/summary.go -- route counts and peer history -->


The UI under `/lg/` uses the same data and adds the peer table, route lookup, route search, and AS-path graph.

## 5. Publish it safely

The looking glass is read-only, but it still publishes topology and routing information. Common deployment choices:

| Deployment | Config |
| --- | --- |
| Public service directly on Ze | `ip 0.0.0.0`, firewall permits TCP/8443 |
| Reverse proxy on the same host | `ip 127.0.0.1`, proxy handles TLS and policy |
| Internal only | bind to a management address and filter at the network edge |

Ze terminates TLS itself unless you set `tls false`. This needs a zefs blob store to hold the certificate, so create one first with `ze init` (see [Build and install Ze on Ubuntu](../ubuntu-build-install/index.md)). Without a blob store, an explicit `tls true` fails with `looking glass TLS requires blob storage (run ze init first)`, while the default falls back to plaintext and prints a warning naming the same remedy.

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

## 6. Operations

```bash
export XDG_RUNTIME_DIR=/run/ze
/usr/local/bin/ze cli -c "show bgp peer list"
/usr/local/bin/ze show warnings
journalctl -u ze.service -n 100 --no-pager
```

If the UI is empty, check that BGP peers are established and that the relevant families are configured. The looking glass can only show routes Ze has learned or originated.
