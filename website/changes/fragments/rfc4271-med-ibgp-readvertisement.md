---
spec: rfc4271-med-ibgp-readvertisement
topic: BGP
feature: egress-attribute-conformance
found-by: rfc-check
---

## Before

A raw export policy returning a full UPDATE body toward an internal BGP peer
could drop MULTI_EXIT_DISC after route selection had already compared MED
across same-neighbor-AS iBGP and eBGP candidates, reopening the route-loop
hazard RFC 4271 Section 9.1.2.2 warns against.

## After

When a selected route carrying MULTI_EXIT_DISC is readvertised over iBGP and a
raw export policy omits attribute 4, Ze restores the selected MED value at the
common egress boundary while keeping unrelated attribute edits from the policy.
Selected routes without MED do not gain one, an explicit egress MED Set still
wins, eBGP MED suppression and route-server transparency are unchanged, and
configured `del { med; }` remains an import-only pre-selection operation.

## Corrects

none

## Exposure

Configurations using external raw export policies on internal BGP peers while
selecting routes carrying MULTI_EXIT_DISC.

## Upstream

none

## Asked by

none
