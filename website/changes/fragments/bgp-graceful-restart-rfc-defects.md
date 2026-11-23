---
spec: bgp-graceful-restart-rfc-defects
topic: BGP
---
## Before

After its own restart, Ze ran route selection as routes arrived, without waiting for its peers to finish re-sending their tables.

## After

With `bgp update-delay max-delay` set, Ze defers route selection per family after its own restart until every peer that advertised Graceful Restart without the Restart State bit has sent End-of-RIB, or until max-delay expires (RFC 4724 Section 4.1). Peers that are themselves restarting, and peers without Graceful Restart, are not waited for. The kernel keeps the routes it preserved until selection finishes.

## Corrects

The 9 November update said that after a restart, route selection waits for End-of-RIB "from restarting peers". It waits for the peers that are not restarting, and only when `bgp update-delay max-delay` is set.

## Exposure

none

## Upstream

none

## Asked by

none
