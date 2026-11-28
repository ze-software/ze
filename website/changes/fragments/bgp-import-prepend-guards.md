---
spec: bgp-import-prepend-guards
topic: BGP
found-by: review
---

## Before

Attaching an active import `policy { modify }` rule carrying `as-path-prepend` to a route-server client (`session { rs-client true; }`) or to a peer declaring `session { asn { local-options [ no-prepend ]; } }` was accepted silently at config validation, startup, and reload. On a route server, the ingress rewrite silently modified `AS_PATH` before relaying the route to other clients (contrary to RFC 7947 Section 2.2.2.1); on a `no-prepend` peer, two contradictory inbound settings remained in force without diagnostic feedback.

## After

`ze config validate`, commit, daemon Stage 2 startup, and live reload now refuse both combinations fail-closed with an operator error naming the offending peer or dynamic group, the modify definition, `as-path-prepend`, the conflicting setting (`rs-client` with RFC 7947 Section 2.2.2.1, or `local-options no-prepend`), and the exact removal fix. Non-`rs-client` peers without `no-prepend`, `replace-as`-only peers, export-chain `as-path-prepend`, and deactivated import references remain accepted.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
