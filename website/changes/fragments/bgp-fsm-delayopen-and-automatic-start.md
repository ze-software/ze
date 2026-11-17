---
spec: bgp-fsm-delayopen-and-automatic-start
topic: BGP
feature: bgp-protocol
found-by: review
---

## Before

Connections waiting to exchange OPEN messages could lose their retry deadline
or an incoming handshake. A competing OPEN could replace the active peer's
learned role, capabilities or address families and interrupt route export.
An UPDATE received before establishment could withdraw the active peer's
routes. Failed receive delivery could also leave read buffers occupied.

## After

The optional DelayOpen timer holds the local OPEN until the peer sends one or
the timer expires, including strict BFD handshakes. Incoming connections keep
their own handshake and original deadlines through selection, startup and read
buffer pressure. Only the selected connection publishes its learned OPEN
state, and UPDATEs reach route consumers only after that connection is
Established. A failed competing exchange preserves the active peer's useful
routes and connection. Failed delivery releases abandoned buffers while
preserving bytes still borrowed by an existing consumer.

## Corrects

none

## Exposure

Peers with concurrent incoming connections or read-buffer pressure could hit
the handshake and resource failures. Competing OPENs could affect the active
peer's learned role, capabilities or address families. DelayOpen and strict
BFD waits also exposed its routes to UPDATEs received before establishment.

## Upstream

none

## Asked by

none
