---
spec: rfc7947-adj-rib-in-accepts-filtered-updates
topic: Route Server
feature: ixp-route-server-dynamic-peers
found-by: rfc-check
---

## Before

A route server client (`rs-client true`) whose UPDATE was denied by ingress loop detection or an import policy chain had its announcement dropped before reaching `bgp-adj-rib-in`, so `show bgp adj-rib-in` showed only post-policy routes and missed RFC 7947 Section 2.1. A mixed UPDATE whose announcements were denied also discarded those announcements before storage, and a denied replacement of a previously accepted route could leave the old accepted route selected.

## After

Route-server client peers (`rs-client true`) now retain ingress-denied UPDATEs (full denials, mixed-UPDATE denied announcements, and denied replacements) pre-policy in `bgp-adj-rib-in` over the registered `update-filtered` event type, marked `filtered: true` with `filter-step` naming the denying ingress step. `show bgp adj-rib-in status` reports `filtered-routes` beside `total-routes`, and the `ze_adjribin_routes_filtered` gauge tracks filtered routes per peer. Import-filtered entries are never selected, forwarded, or replayed on peer-up, a denied replacement withdraws any previously accepted path from Loc-RIB and recipients, RPKI validation cannot promote an import-filtered entry, and unique retained routes stay bounded by configured prefix maximums even under unmatched withdrawals.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
