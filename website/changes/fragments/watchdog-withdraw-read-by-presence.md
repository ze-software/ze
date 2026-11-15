---
spec: watchdog-withdraw-read-by-presence
topic: BGP
---

## Before

A watchdog route with `withdraw false` was held when the BGP session came up.
The watchdog checked whether the setting was present instead of reading its
boolean value.

## After

Watchdog routes announce when the session reaches Established if `withdraw`
is omitted or false. Only `withdraw true` holds a route until an explicit
announce command. Reload keeps the runtime announcement state of existing
routes; new routes use the configured initial state.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
