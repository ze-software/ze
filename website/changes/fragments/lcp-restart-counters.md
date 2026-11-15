---
spec: lcp-restart-counters
topic: PPP
found-by: review
---

## Before

L2TP and PPPoE access servers used fixed LCP retransmission settings. Operators
had no setting for the wait or request limits. Repeated
unacceptable peer options continued to receive Configure-Naks. A PPPoE
client's IPv6CP retry wait was also fixed at three seconds.
After accepted proxy LCP, a peer-triggered restart with no replies ended after
one request instead of using the configured request limit.

## After

The `lcp` block under `l2tp` and `pppoe` configures the Restart timer,
Max-Terminate, Max-Configure and Max-Failure. The timer and request defaults
remain three seconds, two Terminate-Requests and ten Configure-Requests.
Max-Failure defaults to five Naks. Changes apply to new sessions.
Each running session retains its admitted settings.
An accepted proxy LCP session also retains its request budget for a later
peer-triggered restart.

The PPPoE client has configurable LCP timing and limits, plus a separate
`ipv6cp restart-timer`. After the configured Nak limit, unacceptable options
are rejected with their original bytes. A sent Configure-Ack renews that limit.
Late IPv6CP negotiation replies renew the request budget where the state
machine requires it. IPv4 can continue when an unanswered IPv6CP attempt ends.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
