---
spec: bgp-mup-segment-discovery-originator-checks
topic: BGP
feature: typed-family-nlri-discard
found-by: rfc-check
---
## Before

Ze treated received BGP-MUP Direct Segment Discovery (DSD) routes without a Prefix-SID attribute as withdrawn, but accepted Interwork Segment Discovery (ISD) routes without a Prefix-SID and did not verify whether a DSD next hop or an ISD Address field belonged to the IS-IS originator of the route's SRv6 Prefix-SID locator.

## After

Ze now treats both BGP-MUP Direct Segment Discovery (Section 3.3.6) and Interwork Segment Discovery (Section 3.3.3) routes without a Prefix-SID attribute — including after RFC 8669 Section 4 EBGP Prefix-SID discard when `accept-srv6-prefix-sid true` is not configured — as withdrawn per RFC 7606 while keeping sibling routes in the same UPDATE. IS-IS publishes its configured-level SRv6 locator-originator state to BGP across both internal and separate-process deployments; when a DSD next hop (Section 3.3.6) or an ISD Address field (Section 3.3.3, ignoring the received next hop) disagrees with the known locator originator, Ze treats the route as withdrawn and reports `doctor-bgp-mup-locator-mismatch`. When no IGP locator originator can be resolved (such as with no active IS-IS source, no covering locator, or an anycast/re-advertised locator without a Source Router ID), Ze keeps the route unverified and raises a throttled `doctor-bgp-mup-locator-unresolved` warning with guidance to inspect `ze show isis database`.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
