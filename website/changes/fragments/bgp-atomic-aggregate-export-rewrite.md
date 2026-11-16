---
spec: bgp-atomic-aggregate-export-rewrite
topic: BGP
---

## Before

A raw export policy turned a received route carrying ATOMIC_AGGREGATE into
a more-specific route, including when the policy removed the answer's marker.
An accepted policy subset also left an excluded route advertised when the
surviving route failed to encode.

## After

Policy transformations of a received marked route retain its marker and can
keep or remove its original destinations. Attribute edits, reordering and subsets
remain available. Ze refuses new destinations in a marked result, including
less-specific destinations. This restriction also prevents deaggregation.
Independent unmarked routes retain their configured rewrite behaviour.

Excluded destinations pass through the existing ownership checks. A route
owned by the routing table is withdrawn or replaced after route selection.
Another source's route remains protected. Accepted removals survive a later
survivor failure, and quiesce waits for the pending decision and writes.
If cached or native forwarding cannot prepare a required removal, Ze closes
the captured affected session with Cease/Out of Resources. Other destinations and a replacement
connection remain usable.

## Corrects

none

## Exposure

Configurations with raw import or export policies that transform received
marked routes or retain only part of a route group.

## Upstream

none

## Asked by

none
