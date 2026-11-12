---
spec: refuse-instead-of-defaulting
topic: Diagnostics
found-by: review
---

## Before

A failed interface lookup could look like a missing device, and an unavailable
NTP reader could leave `show system date` reporting only the clock. Invalid
`show bgp rib` arguments could return a successful response containing an error.

An NTP client running in an external process could appear inactive because
commands read the daemon's local state. Startup failure could leave its worker
running, and a failed final clock-state save could go unreported at the default
log level.

BMP Loc-RIB monitoring could omit an empty AS_PATH, change AS_SET or multiple
path segments into one sequence, or construct an announcement with no
next hop. An external BMP process could accept Loc-RIB monitoring while
receiving no best-path changes.

Native BGP events and the compact monitor could present a dynamic peer's
unnegotiated AS as AS 0. ADD-PATH identifier 0 could be treated as absent when
routes were indexed or displayed.

## After

Interface lookup failures now stop the operation with the read error. An idle
interface still reports valid zero counters. `show system date` refuses an
unavailable NTP reader while preserving the date-only answer from an inactive
registered provider. Invalid RIB command arguments return an error with a
corrected command.

NTP status, peers and date read the process running the NTP client. Missing,
malformed or unreachable state produces a refusal with the cause and recovery
command. Explicit inactive configuration still permits the date-only answer;
measured unsynchronised state and zero values remain valid. Startup failure
and shutdown stop the worker and release its subscriptions, including when
saving the last-known time fails.

BMP Loc-RIB messages preserve the stored AS_PATH, including its empty form,
segment types and four-octet ASNs. An announcement with no next hop is refused.
Runtime startup and reload refuse Loc-RIB monitoring without its live RIB
feed and identify the internal-plugin configuration that supplies it.

Native BGP events preserve the unknown peer AS before negotiation, and the
compact monitor displays `AS unknown` until the peer's AS is learned. ExaBGP's
existing auto-AS zero spelling is preserved at the compatibility boundary.
ADD-PATH identifier 0 remains a distinct route identity and appears in output
when ADD-PATH is negotiated; a route without ADD-PATH carries no identifier.

## Corrects

none

## Exposure

NTP configurations using an external client process, or commands issued before
the client's state is available or after its transport fails, could receive a
false inactive answer.

BMP senders with `loc-rib true` were affected by locally sourced empty paths,
accepted AS_SET or multiple-segment paths, and best changes without a next hop.
An external BMP instance requesting that feed could start without receiving it.

Dynamic BGP peers before AS negotiation and ADD-PATH families carrying
identifier 0 were affected. Interface operations after a failed backend read
and malformed `show bgp rib` commands could also report misleading results.

## Upstream

none

## Asked by

none
