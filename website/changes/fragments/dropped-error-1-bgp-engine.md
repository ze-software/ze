---
spec: dropped-error-1-bgp-engine
topic: BGP
found-by: review
---

## Before

Several BGP engine failures stayed at debug level or fell back to an empty result. Operators did not see them at the default WARN log level:

- When `ze bgp decode --plugin` ran a broken binary, a closed pipe, or invalid JSON output, it logged at debug level. The command then reported the capability or NLRI as unknown instead of an error.
- When a static route next hop did not resolve, the reactor logged at debug level and skipped the route. Default-originate, queued, and plugin route send failures stayed at debug level too.
- BFD session release failures and BGP event-bus emit failures (End-of-RIB, listener-ready, congestion, and update-notification) logged only at debug level. Refused-batch cache ack failures and malformed UPDATE attribute-index failures did the same. Neither `ze show errors` nor `ze show doctor` reported these faults.

## After

Each failure now reaches the operator:

- `ze bgp decode --plugin` exits non-zero and names the plugin and step when start, pipe I/O, JSON parse, or process exit fails.
- Static-route next-hop failures and route-scoped queued or plugin send failures log a rate-limited WARN under `doctor-bgp-route-not-sent`. The line gives fix `ze show bgp peer list` and raises an event on `ze show errors` and `ze show doctor`.
- Default-originate send failures log a rate-limited WARN under `doctor-bgp-default-originate-not-sent` with fix `ze show bgp peer list` and raise an error event.
- BFD session release failures log a WARN under `doctor-bgp-bfd-unavailable` with fix `ze show bfd sessions` and raise an error event.
- Failed BGP event-bus emits (End-of-RIB, listener-ready, congestion, and update-notification) log a WARN under `doctor-bgp-plugin-events-undelivered` with fix `ze show plugin list` and raise an error event.
- Failed cache acknowledgements on refused batches log a rate-limited WARN under `doctor-bgp-cache-pressure` with fix `ze show cache` and raise an error event.
- Received UPDATE payloads that fail attribute indexing log a rate-limited WARN under `doctor-bgp-malformed-from-peer` with fix `ze show bgp peer list`. The engine raises an error event and keeps wire bytes zero-copy.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
