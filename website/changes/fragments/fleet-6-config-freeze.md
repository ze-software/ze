---
spec: fleet-6-config-freeze
topic: Fleet
feature: fleet-management
---

## Before

A router managed by a hub lost a local change without a trace. An operator who
changed it in an emergency, then restarted it or let the hub change its copy,
found the hub's configuration back in place. Nothing on the router could take
it out of the fleet for a while, and nothing said whether its configuration
still matched what the hub had sent. A hub's SSH session editor also opened,
and committed, the configuration of a client no serving block declared.

## After

A managed router remembers the hash of the last configuration its hub gave it.
When the configuration it runs differs from that hash, it applies nothing the
hub sends, stays connected, tells the hub it is holding, and `ze doctor`
reports `doctor-managed-diverged` with the two ways out: give the router's
configuration to the hub, or roll the router back. The hold ends by itself
once the hub holds the same configuration as the router.

`request fleet disable` takes the router out of the fleet and closes its hub
connection at once; `request fleet enable` brings it back without a restart.
`show fleet status` reports whether the router is managed and frozen, the hub
it follows, both hashes, and from a running daemon whether its hub client runs,
is connected and is holding. All three work on every interface, and with the
daemon stopped.

While a router is managed, a local change to its own configuration is refused
with a message whose fix is `request fleet disable`. The hub's own pushes, the
revert of an unconfirmed commit and boot recovery still apply. A hub's session
editor now refuses a client configuration no serving block declares, with the
same message an import of it gets.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
