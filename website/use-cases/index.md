# Use Cases

Use these pages when you want Ze to play a concrete role in a network.

They are deployment examples: the Ze config, the neighbouring network config, and the lab evidence behind each setup. Feature guides explain every knob. Use-case pages show how the knobs fit together.

## Examples

| Example | Ze role | Network side | Evidence |
| --- | --- | --- | --- |
| [AS112 anycast DNS inside a network](as112/) | Authoritative AS112 DNS sink plus BGP origin for the AS112 covering prefixes | VyOS, Junos, Cisco IOS XR, BIRD, and FRR receive the routes from Ze | Existing AS112 interop scenarios cover the DNS server, BGP redistribution, origin AS, communities, and the covering-prefix guard |
| [ExaBGP migration](exabgp-migration/) | Ze as a replacement engine for an existing ExaBGP deployment | Existing ExaBGP config and process scripts are converted or bridged into Ze | The migration command, compatibility bridge, and staged workflow are linked to implementation sources |
| [BGP performance testing with Ze](bgp-performance/) | Ze provides the sender, receiver, timing, and JSON report for a route-server performance test | FRR, BIRD, Ze, or another DUT receives and exports generated BGP routes | Existing Ze performance tooling supplies the report; the page links to AMS-IX, LINX, and bgperf prior art |
| [Route server at an IXP](route-server/) | RFC 7947 route server with member policy, ADD-PATH, and RPKI validation | IXP members establish one eBGP session and receive eligible member routes | Configuration checks, replay, Adj-RIB-Out inspection, and restart behaviour |
| [Transit edge with RPKI](transit-edge-rpki/) | Dual-transit Internet edge with origin validation and deterministic preference | Two transit providers advertise full or partial tables | RPKI state changes, failover, prefix-limit, memory, and convergence checks |
| [FlowSpec injection](flowspec-injection/) | Authorised source or relay for FlowSpec rules | Mitigation automation and enforcing BGP peers | Commit, propagation, enforcement, expiry, and withdrawal checks |
| [Chaos-tested BGP peering](chaos-tested-peering/) | Peer configuration tested against deterministic failures | A controlled peer or route-server scenario | Recorded seed, replay, property deadlines, shrinking, and recovery checks |
| [AS-path topology](as-path-topology/) | Looking Glass graph for one prefix and its visible paths | Authenticated workbench or public Looking Glass clients | Peer, prefix, route, and graph inspection before and after changes |

## What belongs here

Use-case pages are for complete operator shapes:

- A topology with clear addresses and ASNs.
- A full Ze config for the role.
- The neighboring router or daemon config.
- The verification command or lab scenario that proves the important behavior.

Reference-only material stays in [Documentation](../docs/). Raw interop evidence stays in [Labs](../labs/).