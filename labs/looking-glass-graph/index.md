Demo lab

# Looking Glass Graph Demo

Ze's looking glass, replayed from a real run: Ze collects routes from eleven simulated BGP speakers, and every page and graph below is the answer it gave. A second lab checks the graphs against a hand-built UK network with real AS names.

`Daemon`

The replay comes from a functional test. It generates a network from a seed: five routers of AS65000, one of them a route reflector, and six external neighbours on eBGP. Each speaker holds a real BGP session to Ze and announces the routes a model of the network computed for it. The test asks the looking glass for its pages and its three graphs and compares every answer with the model, router by router, before the recorder keeps a copy for this page.

The UK lab injects 36 routes for a small network: a core ring (Telehouse, Leeds, Manchester, Birmingham) with edge sites (Slough, Bradford) under AS65000, peered with NTT and Cogent for transit and Cloudflare and Akamai for peering. It checks the AS path and next-hop drawings byte for byte, AS names included, which a stub name server supplies. As that stub listens on port 53, this lab runs on Linux only.

- **What this lab tests:** The looking glass pages and its three graphs (AS path, next hop, trace) over real BGP sessions, compared with what a model of the network computed
- **Topology:** Replay: five routers of AS65000 with a route reflector, six external neighbours. UK lab: AS65000 core ring and edge sites, peers NTT, Cogent, Cloudflare and Akamai
- **Requires:** Go toolchain. The UK lab also needs Linux and permission to bind port 53
- **Source:** [test/plugin/lg-simulated-network.ci](https://github.com/ze-software/ze/blob/main/test/plugin/lg-simulated-network.ci), [internal/le/interoplab/bgp/](https://github.com/ze-software/ze/tree/main/internal/le/interoplab/bgp)

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
# the simulated network behind the replay, on any machine
$ ./le test bgp plugin --pattern lg-simulated-network
# the UK lab, Linux only
$ ./le test bgp plugin --pattern lg-graph-lab
```

`Good first lab`

The simulated network needs no Docker, no QEMU and no root. If you want to see Ze's looking glass before running anything else, start here.

- [Looking Glass docs peer and route viewer](https://github.com/ze-software/ze/blob/main/docs/features/looking-glass.md)
