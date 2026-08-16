# AS-Path Topology

Use the built-in Looking Glass to turn the AS paths for a prefix into a topology view. The graph is an operational aid for comparing what different peers advertise, not a complete map of the Internet.

## What the graph shows

For one prefix, Ze collects eligible paths visible through the Looking Glass and builds a directed AS graph. Nodes are AS numbers and edges are observed AS-path adjacencies. Multiple peers may contribute the same edge or different paths to the same origin.

The graph helps answer:

- Which origins are visible for this prefix?
- Which transit sequence did each peer advertise?
- Are several peers showing the same upstream path?
- Did a route leak or unexpected prepend change the shape?
- Does the selected best path match the available alternatives?

## Deploy a complete lab

The graph reads routes through the command dispatcher, so load `bgp-rib` and
enable the read-only Looking Glass. Save this configuration as
`as-path-lab.conf`:

```text
plugin {
    internal rib { use bgp-rib; }
}

environment {
    looking-glass {
        enabled true;
        server main { ip 127.0.0.1; port 8443; }
    }
}

bgp {
    router-id 192.0.2.10;
    session { asn { local 64500; } }

    peer transit-a {
        connection {
            local { ip 192.0.2.10; }
            remote { ip 192.0.2.1; }
        }
        session {
            asn { remote 64496; }
            family {
                ipv4/unicast { prefix { maximum 1000000; } }
            }
        }
    }

    peer transit-b {
        connection {
            local { ip 198.51.100.10; }
            remote { ip 198.51.100.1; }
        }
        session {
            asn { remote 64497; }
            family {
                ipv4/unicast { prefix { maximum 1000000; } }
            }
        }
    }
}
```

Validate and start it, then announce `203.0.113.0/24` from both lab peers with
different AS paths:

```console
ze config validate as-path-lab.conf
ze start as-path-lab.conf
ze cli -c "show bgp rib best prefix 203.0.113.0/24"
curl -fsS "http://127.0.0.1:8443/api/looking-glass/routes/search?prefix=203.0.113.0%2F24"
curl -fsS "http://127.0.0.1:8443/lg/graph?prefix=203.0.113.0%2F24" -o as-path.svg
```

Open `http://127.0.0.1:8443/lg/search?prefix=203.0.113.0%2F24` for the route
details and inline graph. The graph endpoint returns SVG without a client-side
graph engine or external topology service.

## Interpret the result

Read paths from the observing peer toward the origin. Repeated AS numbers may be intentional prepending. Private ASNs, confederation segments, and path sanitisation can change what the graph can display.

Compare the graph with route detail. A graph shows adjacency and path alternatives, while route detail carries local preference, MED, communities, validation state, next hop, and the reason one path won.

## Operator uses

### Investigate a path change

Capture the graph before and after a best-path change. Compare which peer introduced or removed an edge, then inspect that route's attributes and recent BGP events.

### Explain multi-homing

Use the graph to show how two transits reach the same origin. This is often clearer than comparing raw AS-path strings one at a time.

### Check a suspected leak

Look for an unexpected customer or peer appearing between a known transit and origin. Confirm with the raw route, RPKI state, and the announcing peer before treating the drawing as proof.

### Validate route-server visibility

At an IXP, compare paths received from members and confirm the route server has not rewritten the member next hop or AS path.

## Limits

- The graph contains only routes visible to this Ze instance.
- Import-rejected routes are absent unless another retained source exposes them.
- AS-path adjacency does not prove a commercial relationship.
- Aggregation and path hiding can remove intermediate detail.
- A public graph can expose routing information the operator considers sensitive.

Apply the same exposure policy and rate limits as the rest of the public Looking Glass.
