---
spec: rfc4301-architecture-gaps
topic: IPsec
feature: ipsec-data-model
found-by: rfc-check
---

## Before

The IPsec security policy database had no discard action an operator could
write, and an operator bypass entry never decided traffic a gateway forwarded
between a protected host and the outside. An entry that failed to install
still let the commit through. Two overlapping entries left at the same order
were installed in an order nobody chose. Forwarded traffic from UDP source
ports 500 and 4500 left the box in clear, whatever the tunnel selectors said.

## After

A `vpn ipsec policy` entry takes `action discard` or `action bypass`, an
`order` from 1 to 2147483647, a direction, a protocol, ports, and an ICMP
type and code. The entry decides forwarded traffic as well as traffic to and
from Ze, and `show vpn ipsec dataplane policy` lists the forward policies it
installs. A commit is refused when an entry fails to install, and when two
overlapping entries share an order; the refusal names the entry and the
`order` command that fixes it. A port-scoped bypass is refused at commit on a
kernel that cannot reassemble forwarded fragments before the policy check.
Only Ze's own IKE sockets are exempt from the policy database, so forwarded
traffic from ports 500 and 4500 follows the tunnel. ICMP errors carried by a
Child SA are checked against the quoted packet and mapped to the SA that
carries the return traffic. `df-bit clear` and `df-bit copy`, and a `dscp-map`
of copy or 0, set the outer tunnel header; `df-bit set` and a fixed DSCP from
1 to 63 are refused at commit on Linux.

## Corrects

none

## Exposure

A gateway that forwarded traffic for protected hosts was exposed: a host
behind it could send traffic from UDP port 500 or 4500 outside the tunnel,
and a bypass or discard entry did not apply to forwarded traffic. A host that
terminated tunnels only for its own traffic was not exposed to either.

## Upstream

none

## Asked by

none
