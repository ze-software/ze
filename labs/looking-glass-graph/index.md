Demo lab

# Looking Glass Graph Demo

A realistic UK topology with real external ASNs populates the Looking Glass topology graph. The native plugin test suite checks it in a browser.

`Daemon`

Injects 36 routes into a running Ze instance that describes a small but realistic network: a core ring (Telehouse, Leeds, Manchester, Birmingham) with edge sites (Slough, Bradford), peered with real external ASNs: NTT (transit), Cogent (transit), Cloudflare (peering), and Akamai (peering), all under AS65000.

The Go test runner starts a real Looking Glass in an isolated workspace, injects the routes, checks the topology graph and route views, then tears the instance down.

- **What this lab tests:** The Looking Glass topology graph and route views render correctly against a realistic multi-site, multi-peer network
- **Topology:** AS65000 core ring + edge sites, external peers NTT/Cogent/Cloudflare/Akamai
- **Requires:** Go toolchain
- **Source:** [internal/le/interoplab/bgp/](https://github.com/ze-software/ze/tree/main/internal/le/interoplab/bgp)

## The looking glass, replayed

Below is the looking glass itself, not a picture of it: its own pages, stylesheet and scripts, answering from the answers a real ze gave while it served the simulated network. Click a peer, a route or a graph button, or play the tour. Only the recorded questions have an answer, so a search for anything else shows a notice that names the request.

The replay runs in your browser and needs JavaScript. [Read the transcript of the tour](https://ze-software.net/labs/looking-glass-graph/replay/transcript.txt).

### The tour

1. The peers page: every simulated router and external neighbor, each holding a real BGP session to ze, with its uptime, its last state change and how often the session dropped. `GET /lg/peers`
2. One peer's routes: what the external neighbor 203.0.113.1 announced, and how long ago each route last changed. `GET /lg/peer/203.0.113.1 (asked by the page, answered with a fragment)`
3. The search page. `GET /lg/search (asked by the page, answered with a fragment)`
4. Search by prefix: every speaker that announced 198.18.0.0/24, with each route's next hop, AS path and age. `POST /lg/search prefix=198.18.0.0/24 (asked by the page, answered with a fragment)`
5. The AS path graph of 198.18.0.0/24: the ASes its routes cross. `GET /lg/graph?prefix=198.18.0.0/24&mode=aspath (asked by the page, answered with a fragment)`
6. The next-hop graph of 198.18.0.0/24: which router hands the traffic to which. Routers of the AS show by address, the neighbors outside by AS number only. `GET /lg/graph?prefix=198.18.0.0/24&mode=nexthop (asked by the page, answered with a fragment)`
7. The trace of 198.18.0.0/24: each router's BGP next hop followed router to router to the exit, then the AS it is handed to and the AS path to the origin. From 198.51.100.1 it crosses three routers of the AS. `GET /lg/graph?prefix=198.18.0.0/24&mode=trace (asked by the page, answered with a fragment)`
8. The trace as text, the form a test reads. `GET /lg/graph?prefix=198.18.0.0/24&mode=trace&format=text`
9. The AS path graph as text. `GET /lg/graph?prefix=198.18.0.0/24&mode=aspath&format=text`
10. The next-hop graph as text. `GET /lg/graph?prefix=198.18.0.0/24&mode=nexthop&format=text`
11. Route detail: the route 203.0.113.1 sent for 198.18.0.0/24, with every attribute and its age. `GET /lg/route/detail?prefix=198.18.0.0/24&peer=203.0.113.1 (asked by the page, answered with a fragment)`
12. Search by address: the routes that cover 198.18.0.1. `POST /lg/search prefix=198.18.0.1 (asked by the page, answered with a fragment)`
13. Search by AS path with the end anchor: 64499$ lists the routes AS 64499 originates. `POST /lg/search aspath=64499$ (asked by the page, answered with a fragment)`

Recorded by ./le site lg-replay record from commit bad92271a4 (2026-11-19): a real ze, 11 simulated BGP speakers generated from seed 7, 84 recorded answers. 234 values that change from run to run (session uptimes, state-change times, the listener address) are fixed values the recording declares. None of the networks shown is a real one: the prefixes come from 198.18.0.0/15, the range RFC 2544 reserves for benchmarking, the speakers use documentation addresses (RFC 5737), and the AS numbers are private or documentation numbers (RFC 6996, RFC 5398). [Transcript of the tour](https://ze-software.net/labs/looking-glass-graph/replay/transcript.txt).

```
# runs the native plugin suite, including the graph lab
$ ./le functional plugin-test
```

`Good first lab`

No Docker, no QEMU, no root. If you want to see what Ze's management interfaces look like before running anything else, start here.

- [Looking Glass docs peer and route viewer](https://github.com/ze-software/ze/blob/main/docs/features/looking-glass.md)
