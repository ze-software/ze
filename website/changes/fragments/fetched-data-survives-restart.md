---
spec: fetched-data-survives-restart
topic: BGP
feature: irr-bgp-import-filtering
---

## Before

A reboot could change what the router enforced. The BGP IRR import filter kept
its prefix lists in memory only, so every start asked the IRR server again:
until each answer arrived, announcements from IRR-filtered peers were rejected
and held, and the new lists could differ from the ones enforced before the
reboot. With the IRR server unreachable at boot, those peers stayed rejected.
BGP and the firewall each fetched the same AS-SET on their own schedule, from
settings written twice (`policy irr` and `firewall irr`, with different default
refresh periods). Every firewall domain-group name was resolved again at start,
whatever its TTL.

## After

One `irr` plugin fetches, stores and refreshes every IRR list, configured once
in `system irr { server; source-address; refresh-interval }` (default 3600
seconds, 0 for manual only) with the PeeringDB URL in `system peeringdb url`.
At start, BGP and the firewall enforce the stored lists at once and no IRR
query is sent; a list is fetched again only when its refresh period has passed
since the server last answered, or on `update irr all`, `update irr as-set
<name>` or `update irr asn <asn>`. `clear irr as-set <name>` removes a stored
list and names the consumers still using it. `show irr` reports each stored
list and its next refresh. The old `update bgp irr`, `update firewall irr` and
`clear firewall irr` commands and the `policy irr` and `firewall irr` server,
URL and interval settings are gone. A domain-group name is resolved at start
only once its stored TTL has expired.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
